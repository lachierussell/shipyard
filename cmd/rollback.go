package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/shipyard/shipyard/config"
	"github.com/shipyard/shipyard/update"
)

// Rollback restores the previous binary from backup
func Rollback() error {
	// Find config file
	configPath := "/usr/local/etc/shipyard/shipyard.toml"

	// For development, also check current directory
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "shipyard.toml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	updater := update.NewUpdater(cfg.Self.BinaryPath)

	if !updater.HasBackup() {
		return fmt.Errorf("no backup binary found at %s.old", cfg.Self.BinaryPath)
	}

	slog.Info("rolling back to previous binary", "path", cfg.Self.BinaryPath+".old")

	if err := updater.Rollback(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	slog.Info("rollback successful")
	fmt.Println("Rollback successful!")
	fmt.Println("Note: You may need to restart the shipyard service for changes to take effect:")
	fmt.Println("  service shipyard restart")

	return nil
}
