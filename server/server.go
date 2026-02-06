package server

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/lachierussell/shipyard/config"
	"github.com/lachierussell/shipyard/deploy"
	"github.com/lachierussell/shipyard/jail"
	"github.com/lachierussell/shipyard/nginx"
	"github.com/lachierussell/shipyard/service"
	"github.com/lachierussell/shipyard/ssl"
	"github.com/lachierussell/shipyard/update"
)

// MaxRequestSize is the maximum allowed request body size (500MB)
const MaxRequestSize = 500 << 20

// Server manages the HTTP API server
type Server struct {
	app              *fiber.App
	cfg              *config.Config
	version          string
	commit           string
	nginxMgr         *nginx.Manager
	jailMgr          *jail.Manager
	serviceMgr       *service.Manager
	sslMgr           *ssl.Manager
	frontendDeployer *deploy.FrontendDeployer
	backendDeployer  *deploy.BackendDeployer
	updater          *update.Updater
	logHub           *LogHub
	shutdownChan     chan struct{}
}

// New creates a new HTTP server with routes configured.
// logHub may be nil if log streaming is not needed.
func New(cfg *config.Config, version, commit string, logHub *LogHub) *Server {
	app := fiber.New(fiber.Config{
		Prefork:   false,
		BodyLimit: MaxRequestSize,
	})

	// Global middleware
	app.Use(CORS())
	app.Use(SizeLimit())
	app.Use(RequestLogger())

	srv := &Server{
		app:              app,
		cfg:              cfg,
		version:          version,
		commit:           commit,
		nginxMgr:         nginx.NewManager(cfg),
		jailMgr:          jail.NewManager(cfg),
		serviceMgr:       service.NewManager(cfg),
		sslMgr:           ssl.NewManager(cfg),
		frontendDeployer: deploy.NewFrontendDeployer(cfg),
		backendDeployer:  deploy.NewBackendDeployer(cfg),
		updater:          update.NewUpdater(cfg.Self.BinaryPath),
		logHub:           logHub,
		shutdownChan:     make(chan struct{}),
	}
	srv.setupRoutes()

	return srv
}

// setupRoutes registers all API routes
func (s *Server) setupRoutes() {
	// Health checks (no auth)
	s.app.Get("/health", s.Health)
	s.app.Get("/status/:site", s.Status)

	// Site lifecycle (admin auth)
	s.app.Get("/sites", AdminAuth(s.cfg), s.ListSites)
	s.app.Post("/site/create", AdminAuth(s.cfg), s.SiteCreate)
	s.app.Post("/site/init", AdminAuth(s.cfg), s.SiteInit)
	s.app.Post("/site/destroy", AdminAuth(s.cfg), s.SiteDestroy)

	// Site info (admin auth)
	s.app.Get("/site/logs", AdminAuth(s.cfg), s.SiteLogs)

	// Nginx config helpers (admin auth)
	s.app.Get("/nginx/example", AdminAuth(s.cfg), s.NginxExample)

	// Deploy endpoints (per-site auth)
	s.app.Post("/deploy/frontend", SiteAuth(s.cfg), s.DeployFrontend)
	s.app.Post("/deploy/backend", SiteAuth(s.cfg), s.DeployBackend)
	s.app.Post("/deploy/self", AdminAuth(s.cfg), s.DeploySelf)

	// WebSocket log streaming (admin auth via query param)
	if s.logHub != nil {
		s.app.Use("/ws", s.WSLogsUpgrade)
		s.app.Get("/ws/logs", websocket.New(s.WSLogs))
	}
}

// Listen starts the HTTP server
func (s *Server) Listen(addr string) error {
	// Use TLS if cert and key are configured
	if s.cfg.Server.TLSCert != "" && s.cfg.Server.TLSKey != "" {
		return s.app.ListenTLS(addr, s.cfg.Server.TLSCert, s.cfg.Server.TLSKey)
	}
	return s.app.Listen(addr)
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown() error {
	if s.logHub != nil {
		s.logHub.Stop()
	}
	return s.app.Shutdown()
}

// ShutdownChan returns the channel used to signal shutdown for self-update
func (s *Server) ShutdownChan() <-chan struct{} {
	return s.shutdownChan
}

// TriggerShutdown signals the server to shutdown (used after self-update)
func (s *Server) TriggerShutdown() {
	close(s.shutdownChan)
}

// reqLog returns the request-scoped logger stored by the RequestLogger middleware.
// Falls back to the default logger if not present.
func reqLog(c *fiber.Ctx) *slog.Logger {
	if l, ok := c.Locals("logger").(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

