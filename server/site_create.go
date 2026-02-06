package server

import (
	"path/filepath"
	"regexp"

	"github.com/gofiber/fiber/v2"
	"github.com/shipyard/shipyard/config"
	"github.com/shipyard/shipyard/nginx"
	"github.com/shipyard/shipyard/ssl"
)

var validDomain = regexp.MustCompile(`^[a-z0-9][a-z0-9\.\-]*[a-z0-9]$`)

// SiteCreateRequest is the JSON body for creating a new site
type SiteCreateRequest struct {
	Domain       string `json:"domain"`
	FrontendRoot string `json:"frontend_root,omitempty"`
	SSLEnabled   bool   `json:"ssl_enabled"`
	WithBackend  bool   `json:"with_backend"`
	BackendPort  int    `json:"backend_port,omitempty"`
	ProxyPath    string `json:"proxy_path,omitempty"`
}

// SiteCreate creates a new site configuration and generates an API key
func (s *Server) SiteCreate(c *fiber.Ctx) error {
	log := reqLog(c)

	var req SiteCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_request",
			"detail": "failed to parse JSON body",
		})
	}

	// Validate required fields
	if req.Domain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "missing_domain",
		})
	}
	if !validDomain.MatchString(req.Domain) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "invalid_domain",
			"detail": "domain must be lowercase alphanumeric with dots and hyphens",
		})
	}

	// Check if site already exists
	if _, exists := s.cfg.Site[req.Domain]; exists {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status": "error",
			"error":  "site_exists",
		})
	}

	// Generate API key
	apiKey, err := config.GenerateAPIKey("sk-site-")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error",
			"error":  "key_generation_failed",
		})
	}

	// Set defaults for frontend root
	// Backend-only: with_backend=true and no frontend_root means no frontend
	frontendRoot := req.FrontendRoot
	backendOnly := req.WithBackend && frontendRoot == ""
	if !backendOnly && frontendRoot == "" {
		frontendRoot = filepath.Join("/var/www", req.Domain)
	}

	log = log.With("domain", req.Domain, "ssl", req.SSLEnabled, "with_backend", req.WithBackend, "backend_only", backendOnly)
	log.Info("site creation started")

	site := config.SiteConfig{
		FrontendRoot: frontendRoot,
		APIKey:       apiKey,
		SSLEnabled:   req.SSLEnabled,
	}

	// Add backend config if requested
	if req.WithBackend {
		port := req.BackendPort
		if port == 0 {
			port = 8080
		}
		proxyPath := req.ProxyPath
		if proxyPath == "" {
			proxyPath = "/api"
		}

		site.Backend = &config.BackendConfig{
			JailName:   req.Domain,
			JailIP:     s.cfg.NextJailIP(),
			ListenPort: port,
			ProxyPath:  proxyPath,
			BinaryName: req.Domain,
		}
	}

	// Generate SSL certificate BEFORE saving config
	// This ensures we don't end up with a site that has ssl_enabled but no cert
	if req.SSLEnabled {
		// Step 1: Deploy temporary HTTP-only nginx config for ACME challenge
		if err := s.nginxMgr.DeployHTTPOnlyConfig(req.Domain); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error",
				"error":  "nginx_setup_failed",
				"detail": err.Error(),
			})
		}

		// Step 2: Obtain Let's Encrypt certificate via webroot
		if err := s.sslMgr.ObtainCert(req.Domain); err != nil {
			// Clean up the temporary nginx config on failure
			s.nginxMgr.RemoveSiteConfigByDomain(req.Domain)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error",
				"error":  "cert_generation_failed",
				"detail": err.Error(),
			})
		}
		// Note: The HTTP-only config remains until the site is fully initialized
		// At that point, DeploySiteConfig will replace it with the full SSL config
	}

	// Add site to config and save (domain is the key)
	if err := s.cfg.AddSite(req.Domain, site); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error",
			"error":  "save_failed",
			"detail": err.Error(),
		})
	}

	// Deploy nginx config for backend if present
	nginxDeployed := false
	if site.Backend != nil {
		var nginxConfig string
		if backendOnly {
			// Backend-only: use backend proxy template (no frontend root)
			if req.SSLEnabled {
				certPath, keyPath := ssl.CertPaths(req.Domain)
				nginxConfig = nginx.GenerateBackendProxyConfigHTTPS(req.Domain, site.Backend.ListenPort, site.Backend.ProxyPath, certPath, keyPath)
			} else {
				nginxConfig = nginx.GenerateBackendProxyConfig(req.Domain, site.Backend.ListenPort, site.Backend.ProxyPath)
			}
		} else {
			// Combined: frontend + backend proxy template
			if req.SSLEnabled {
				certPath, keyPath := ssl.CertPaths(req.Domain)
				nginxConfig = nginx.GenerateSiteCombinedConfigHTTPS(req.Domain, frontendRoot, site.Backend.ListenPort, site.Backend.ProxyPath, certPath, keyPath)
			} else {
				nginxConfig = nginx.GenerateSiteCombinedConfig(req.Domain, frontendRoot, site.Backend.ListenPort, site.Backend.ProxyPath)
			}
		}

		// Deploy directly to sites-available and reload
		reloaded, nginxErr, err := s.nginxMgr.DeploySiteConfigRaw(req.Domain, nginxConfig)
		if err != nil {
			_ = nginxErr
		} else {
			nginxDeployed = reloaded
		}
	}

	log.Info("site created", "nginx_deployed", nginxDeployed)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":         "created",
		"domain":         req.Domain,
		"api_key":        apiKey,
		"frontend_root":  frontendRoot,
		"ssl_enabled":    req.SSLEnabled,
		"has_backend":    req.WithBackend,
		"backend_only":   backendOnly,
		"nginx_deployed": nginxDeployed,
	})
}
