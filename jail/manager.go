package jail

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/lachierussell/shipyard/config"
)

// Manager handles jail lifecycle operations using pot
type Manager struct {
	cfg *config.Config
}

// NewManager creates a new jail manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// potCmd returns the configured pot binary path, falling back to "pot" if unset.
func (m *Manager) potCmd() string {
	if m.cfg.Jail.BinaryPath != "" {
		return m.cfg.Jail.BinaryPath
	}
	return "pot"
}

// potName converts a site name to a valid pot name (alphanumeric and hyphens only)
func potName(siteName string) string {
	// Replace dots with hyphens for pot compatibility
	return strings.ReplaceAll(siteName, ".", "-")
}

// EnsureExists creates a pot if it doesn't exist (idempotent)
func (m *Manager) EnsureExists(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return fmt.Errorf("site %s has no backend config", siteName)
	}

	name := potName(siteName)

	// Check if pot already exists
	if m.potExists(name) {
		return nil
	}

	// Create pot
	slog.Info("creating pot", "site", siteName, "pot", name)
	return m.createPot(siteName)
}

// potExists checks if a pot with the given name exists
func (m *Manager) potExists(name string) bool {
	cmd := exec.Command(m.potCmd(), "info", "-p", name)
	return cmd.Run() == nil
}

// createPot creates a new pot for a site
func (m *Manager) createPot(siteName string) error {
	name := potName(siteName)

	// Create a pot based on the default base
	// -t single: single ZFS dataset
	// -b: base pot to use (default FreeBSD base)
	// -N inherit: share host network stack (allows outbound connections)
	args := []string{
		"create",
		"-p", name,
		"-t", "single",
		"-b", m.cfg.Jail.FreeBSDVersion,
		"-N", "inherit",
	}

	cmd := exec.Command(m.potCmd(), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pot create: %w: %s", err, string(output))
	}

	return nil
}

// Start starts a pot
func (m *Manager) Start(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return fmt.Errorf("site %s has no backend config", siteName)
	}

	name := potName(siteName)
	slog.Info("starting pot", "site", siteName, "pot", name)
	cmd := exec.Command(m.potCmd(), "start", "-p", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pot start: %w: %s", err, string(output))
	}
	return nil
}

// Stop stops a pot
func (m *Manager) Stop(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil
	}

	name := potName(siteName)
	slog.Debug("stopping pot", "site", siteName, "pot", name)
	cmd := exec.Command(m.potCmd(), "stop", "-p", name)
	if err := cmd.Run(); err != nil {
		slog.Debug("pot stop failed (may not be running)", "site", siteName, "error", err)
	}
	return nil
}

// Destroy removes a pot completely
func (m *Manager) Destroy(siteName string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil
	}

	name := potName(siteName)
	slog.Info("destroying pot", "site", siteName, "pot", name)

	// Stop pot first
	m.Stop(siteName)

	// Destroy pot
	cmd := exec.Command(m.potCmd(), "destroy", "-p", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pot destroy: %w: %s", err, string(output))
	}

	return nil
}

// CopyIn copies a file into the pot
func (m *Manager) CopyIn(siteName string, srcPath string, destPath string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return fmt.Errorf("site %s has no backend config", siteName)
	}

	name := potName(siteName)
	// Use -F flag to allow copying to a running pot
	cmd := exec.Command(m.potCmd(), "copy-in", "-p", name, "-F", "-s", srcPath, "-d", destPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pot copy-in: %w: %s", err, string(output))
	}

	return nil
}

// Exec executes a command inside the pot
func (m *Manager) Exec(siteName string, command string, args ...string) error {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return fmt.Errorf("site %s has no backend config", siteName)
	}

	name := potName(siteName)

	// Build the pot exec command
	execArgs := []string{"exec", "-p", name, command}
	execArgs = append(execArgs, args...)

	cmd := exec.Command(m.potCmd(), execArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pot exec: %w: %s", err, string(output))
	}

	return nil
}

// IsRunning checks if a pot is running
func (m *Manager) IsRunning(siteName string) bool {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return false
	}

	if site.Backend == nil {
		return false
	}

	name := potName(siteName)

	// pot ps lists running pots
	cmd := exec.Command(m.potCmd(), "ps", "-q")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if our pot is in the list
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == name {
			return true
		}
	}

	return false
}

// GetPotPath returns the filesystem path to a pot
func (m *Manager) GetPotPath(siteName string) (string, error) {
	site, ok := m.cfg.Site[siteName]
	if !ok {
		return "", fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return "", fmt.Errorf("site %s has no backend config", siteName)
	}

	name := potName(siteName)

	// Get pot info to find the path
	cmd := exec.Command(m.potCmd(), "info", "-p", name, "-E")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pot info: %w", err)
	}

	// Parse output to find pot-path
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "pot-path:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "pot-path:")), nil
		}
	}

	// Fallback: construct the path based on pot conventions
	return fmt.Sprintf("/opt/pot/jails/%s", name), nil
}
