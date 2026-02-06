package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lachierussell/shipyard/config"
	"github.com/lachierussell/shipyard/logger"
	"github.com/lachierussell/shipyard/pidfile"
	"github.com/lachierussell/shipyard/server"
	"github.com/lachierussell/shipyard/update"
)

// Serve starts the HTTP server
func Serve(version, commit string) error {
	// Find config file (look for /usr/local/etc/shipyard/shipyard.toml by default)
	configPath := "/usr/local/etc/shipyard/shipyard.toml"

	// For development, also check current directory
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "shipyard.toml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create log hub for WebSocket streaming
	logHub := server.NewLogHub()
	go logHub.Run()

	// Initialize structured logging with broadcast to WebSocket clients
	if err := logger.InitWithBroadcaster(cfg.Server.LogFile, cfg.Server.LogLevel, logHub); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	// Startup safety check: warn if backup binary exists
	checkBackupBinary(cfg.Self.BinaryPath)

	// Create PID file (single-instance enforcement)
	pf, err := pidfile.Create(cfg.Self.PidFile)
	if err != nil {
		return fmt.Errorf("pidfile: %w", err)
	}
	defer pf.Close()

	// Create server
	srv := server.New(cfg, version, commit, logHub)

	slog.Info("server starting",
		"version", version,
		"commit", commit,
		"listen_addr", cfg.Server.ListenAddr,
	)

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Listen(cfg.Server.ListenAddr); err != nil {
			errChan <- fmt.Errorf("listen: %w", err)
		}
	}()

	// Wait for shutdown signal, self-update trigger, or error
	select {
	case sig := <-sigChan:
		slog.Info("received signal, shutting down", "signal", sig.String())
	case <-srv.ShutdownChan():
		slog.Info("self-update triggered, shutting down for restart")
	case err := <-errChan:
		return err
	}

	// Graceful shutdown
	if err := srv.Shutdown(); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	slog.Info("shutdown complete")
	return nil
}

// checkBackupBinary logs a notice if a backup binary exists from a previous update
func checkBackupBinary(binaryPath string) {
	updater := update.NewUpdater(binaryPath)
	if updater.HasBackup() {
		slog.Warn("backup binary exists from previous update",
			"path", binaryPath+".old",
			"hint", "run 'shipyard rollback' to restore the previous version",
		)
	}
}
