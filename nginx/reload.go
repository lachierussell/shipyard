package nginx

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/shipyard/shipyard/config"
	"github.com/shipyard/shipyard/ssl"
)

// Manager orchestrates the nginx config deployment: write → validate → symlink → reload
type Manager struct {
	cfg *config.Config
}

// NewManager creates a new nginx manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// DeploySiteConfig writes a site config to sites-available, validates it, and reloads nginx
// If SSL is enabled for the site, it transforms the config to HTTPS
func (m *Manager) DeploySiteConfig(siteName string, nginxConfig string) (bool, string, error) {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return false, "", fmt.Errorf("site not found: %s", siteName)
	}

	// If SSL is enabled, transform the config to HTTPS with HTTP redirect
	// Note: siteName IS the domain (domain is the key)
	finalConfig := nginxConfig
	if site.SSLEnabled {
		certPath, keyPath := ssl.CertPaths(siteName)
		finalConfig = TransformToHTTPS(nginxConfig, siteName, certPath, keyPath)
	}

	return m.DeploySiteConfigRaw(siteName, finalConfig)
}

// DeploySiteConfigRaw writes a site config directly without SSL transformation
// Use this when the config already includes SSL directives (e.g., from HTTPS templates)
func (m *Manager) DeploySiteConfigRaw(siteName string, nginxConfig string) (bool, string, error) {
	if _, ok := m.cfg.Site[siteName]; !ok {
		return false, "", fmt.Errorf("site not found: %s", siteName)
	}

	// Ensure sites-available directory exists
	if err := os.MkdirAll(m.cfg.Nginx.SitesAvailable, 0755); err != nil {
		return false, "", fmt.Errorf("mkdir sites-available: %w", err)
	}

	// Write site config to sites-available
	siteConfPath := filepath.Join(m.cfg.Nginx.SitesAvailable, siteName+".conf")
	if err := os.WriteFile(siteConfPath, []byte(nginxConfig), 0644); err != nil {
		return false, "", fmt.Errorf("write site config: %w", err)
	}

	// Regenerate override.conf
	overrideContent := GenerateOverrideConf(m.cfg)
	if err := os.WriteFile(m.cfg.Nginx.OverrideConf, []byte(overrideContent), 0644); err != nil {
		return false, "", fmt.Errorf("write override conf: %w", err)
	}

	// Validate the entire nginx config
	isValid, errMsg := ValidateAndGetError(m.cfg)
	if !isValid {
		slog.Warn("nginx validation failed", "domain", siteName, "error", errMsg)
		return false, errMsg, nil
	}

	// Validation passed: symlink and reload
	if err := m.symlinkSiteConfig(siteName); err != nil {
		return false, "", fmt.Errorf("symlink site config: %w", err)
	}

	// Reload nginx
	slog.Info("reloading nginx", "domain", siteName)
	cmd := exec.Command(m.cfg.Nginx.BinaryPath, "-s", "reload")
	if err := cmd.Run(); err != nil {
		return false, "", fmt.Errorf("nginx reload: %w", err)
	}

	return true, "", nil
}

// symlinkSiteConfig creates a symlink from sites-enabled to sites-available
func (m *Manager) symlinkSiteConfig(siteName string) error {
	// siteName IS the domain (domain is the key)
	if _, ok := m.cfg.Site[siteName]; !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if err := os.MkdirAll(m.cfg.Nginx.SitesEnabled, 0755); err != nil {
		return fmt.Errorf("mkdir sites-enabled: %w", err)
	}

	availablePath := filepath.Join(m.cfg.Nginx.SitesAvailable, siteName+".conf")
	enabledPath := filepath.Join(m.cfg.Nginx.SitesEnabled, siteName+".conf")

	// Remove existing symlink if present
	os.Remove(enabledPath)

	// Create symlink: enabled -> available
	if err := os.Symlink(availablePath, enabledPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	return nil
}

// DeployHTTPOnlyConfig deploys a minimal HTTP config for ACME challenge
// This is used during initial certificate acquisition
func (m *Manager) DeployHTTPOnlyConfig(domain string) error {
	// Ensure directories exist
	if err := os.MkdirAll(m.cfg.Nginx.SitesAvailable, 0755); err != nil {
		return fmt.Errorf("mkdir sites-available: %w", err)
	}
	if err := os.MkdirAll(m.cfg.Nginx.SitesEnabled, 0755); err != nil {
		return fmt.Errorf("mkdir sites-enabled: %w", err)
	}

	// Generate HTTP-only config
	httpConfig := GenerateHTTPOnlyConfig(domain)

	// Write to sites-available
	availablePath := filepath.Join(m.cfg.Nginx.SitesAvailable, domain+".conf")
	if err := os.WriteFile(availablePath, []byte(httpConfig), 0644); err != nil {
		return fmt.Errorf("write http config: %w", err)
	}

	// Symlink to sites-enabled
	enabledPath := filepath.Join(m.cfg.Nginx.SitesEnabled, domain+".conf")
	os.Remove(enabledPath) // remove if exists
	if err := os.Symlink(availablePath, enabledPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	// Validate nginx config
	isValid, errMsg := ValidateAndGetError(m.cfg)
	if !isValid {
		// Clean up on failure
		os.Remove(enabledPath)
		os.Remove(availablePath)
		return fmt.Errorf("nginx validation failed: %s", errMsg)
	}

	// Reload nginx
	cmd := exec.Command(m.cfg.Nginx.BinaryPath, "-s", "reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nginx reload: %w", err)
	}

	return nil
}

// RemoveSiteConfigByDomain removes a site's nginx config by domain name
// Does not check if site exists in config (useful for cleanup)
func (m *Manager) RemoveSiteConfigByDomain(domain string) error {
	enabledPath := filepath.Join(m.cfg.Nginx.SitesEnabled, domain+".conf")
	availablePath := filepath.Join(m.cfg.Nginx.SitesAvailable, domain+".conf")

	os.Remove(enabledPath)
	os.Remove(availablePath)

	// Reload nginx
	cmd := exec.Command(m.cfg.Nginx.BinaryPath, "-s", "reload")
	cmd.Run() // ignore errors

	return nil
}

// RemoveSiteConfig removes a site from nginx and reloads
func (m *Manager) RemoveSiteConfig(siteName string) error {
	// siteName IS the domain (domain is the key)
	if _, ok := m.cfg.Site[siteName]; !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	// Remove from sites-enabled and sites-available
	enabledPath := filepath.Join(m.cfg.Nginx.SitesEnabled, siteName+".conf")
	availablePath := filepath.Join(m.cfg.Nginx.SitesAvailable, siteName+".conf")

	os.Remove(enabledPath)
	os.Remove(availablePath)

	// Regenerate override.conf (without this site)
	overrideContent := GenerateOverrideConf(m.cfg)
	if err := os.WriteFile(m.cfg.Nginx.OverrideConf, []byte(overrideContent), 0644); err != nil {
		return fmt.Errorf("write override conf: %w", err)
	}

	// Validate and reload
	isValid, errMsg := ValidateAndGetError(m.cfg)
	if !isValid {
		return fmt.Errorf("nginx validation after removal: %s", errMsg)
	}

	cmd := exec.Command(m.cfg.Nginx.BinaryPath, "-s", "reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nginx reload: %w", err)
	}

	return nil
}
