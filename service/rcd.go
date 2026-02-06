package service

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/lachierussell/shipyard/config"
)

type rcdData struct {
	ServiceName string
	PotName     string
	BinaryPath  string
	ListenPort  int
}

//go:embed rcd.sh.tmpl
var rcdTmplStr string

var rcdTmpl = template.Must(template.New("rcd").Delims("<%", "%>").Parse(rcdTmplStr))

// Manager manages rc.d service scripts for pot-based services
type Manager struct {
	cfg *config.Config
}

// NewManager creates a new service manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// potName converts a site name to a valid pot name (alphanumeric and hyphens only)
func potName(siteName string) string {
	return strings.ReplaceAll(siteName, ".", "-")
}

// serviceName converts a site name to a valid service name (underscores for rc.d)
func serviceName(siteName string) string {
	name := strings.ReplaceAll(siteName, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

// CreateBackendService creates an rc.d script for a pot-based backend service
func (m *Manager) CreateBackendService(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return fmt.Errorf("site %s has no backend config", siteName)
	}

	potN := potName(siteName)
	svcName := serviceName(siteName)
	binaryPath := filepath.Join("/usr/local/bin", site.Backend.BinaryName)
	listenPort := site.Backend.ListenPort

	var buf bytes.Buffer
	if err := rcdTmpl.Execute(&buf, rcdData{
		ServiceName: svcName,
		PotName:     potN,
		BinaryPath:  binaryPath,
		ListenPort:  listenPort,
	}); err != nil {
		return fmt.Errorf("execute rcd template: %w", err)
	}
	scriptContent := buf.String()

	// Write rc.d script to the actual location
	rcdPath := filepath.Join("/usr/local/etc/rc.d", svcName)
	if err := os.MkdirAll(filepath.Dir(rcdPath), 0755); err != nil {
		return fmt.Errorf("mkdir rc.d: %w", err)
	}

	if err := os.WriteFile(rcdPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("write rc.d script: %w", err)
	}

	return nil
}

// RemoveBackendService removes an rc.d script
func (m *Manager) RemoveBackendService(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil // No backend to remove
	}

	svcName := serviceName(siteName)
	rcdPath := filepath.Join("/usr/local/etc/rc.d", svcName)
	os.Remove(rcdPath)

	return nil
}

// Enable enables a service
func (m *Manager) Enable(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil
	}

	slog.Info("enabling service", "site", siteName)
	return enableService(serviceName(siteName))
}

// Disable disables a service
func (m *Manager) Disable(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil
	}

	slog.Info("disabling service", "site", siteName)
	return disableService(serviceName(siteName))
}

// Start starts a service
func (m *Manager) Start(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil
	}

	slog.Info("starting service", "site", siteName)
	return startService(serviceName(siteName))
}

// Stop stops a service
func (m *Manager) Stop(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil
	}

	slog.Info("stopping service", "site", siteName)
	stopService(serviceName(siteName))
	return nil
}

// Restart restarts a service
func (m *Manager) Restart(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil
	}

	return restartService(serviceName(siteName))
}

// Status checks service status
func (m *Manager) Status(siteName string) (bool, error) {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return false, fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return false, nil
	}

	return checkService(serviceName(siteName)), nil
}
