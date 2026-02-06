package ssl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lachierussell/shipyard/config"
)

// AcmeWebroot is the directory where ACME challenges are served from
const AcmeWebroot = "/var/www/acme"

// Manager handles SSL certificate generation via Let's Encrypt
type Manager struct {
	cfg *config.Config
}

// NewManager creates a new SSL manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// CertPaths returns the paths to the SSL certificate and key for a domain
func CertPaths(domain string) (certPath, keyPath string) {
	base := filepath.Join("/usr/local/etc/letsencrypt/live", domain)
	return filepath.Join(base, "fullchain.pem"), filepath.Join(base, "privkey.pem")
}

// HasValidCert checks if a valid certificate already exists for the domain
func (m *Manager) HasValidCert(domain string) bool {
	certPath, keyPath := CertPaths(domain)
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	return certErr == nil && keyErr == nil
}

// ObtainCert obtains a Let's Encrypt certificate for a domain using webroot method
// Requires nginx to be configured to serve /.well-known/acme-challenge from AcmeWebroot
func (m *Manager) ObtainCert(domain string) error {
	// Skip if cert already exists
	if m.HasValidCert(domain) {
		return nil
	}

	// Ensure ACME webroot directory exists
	acmeDir := filepath.Join(AcmeWebroot, ".well-known", "acme-challenge")
	if err := os.MkdirAll(acmeDir, 0755); err != nil {
		return fmt.Errorf("create acme directory: %w", err)
	}

	// Run certbot with webroot
	cmd := exec.Command("certbot", "certonly",
		"--webroot",
		"--webroot-path", AcmeWebroot,
		"--non-interactive",
		"--agree-tos",
		"--register-unsafely-without-email",
		"-d", domain,
	)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("certbot failed: %w\nOutput: %s", err, string(output))
	}

	// Verify cert was created
	if !m.HasValidCert(domain) {
		return fmt.Errorf("certbot succeeded but certificate not found at expected path")
	}

	return nil
}

// RenewAll renews all certificates that are close to expiry
func (m *Manager) RenewAll() error {
	cmd := exec.Command("certbot", "renew",
		"--webroot",
		"--webroot-path", AcmeWebroot,
		"--quiet",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot renew failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
