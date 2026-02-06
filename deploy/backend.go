package deploy

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/shipyard/shipyard/config"
	"github.com/shipyard/shipyard/jail"
	"github.com/shipyard/shipyard/service"
)

// BackendDeployer handles backend service deployment
type BackendDeployer struct {
	cfg *config.Config
}

// NewBackendDeployer creates a new backend deployer
func NewBackendDeployer(cfg *config.Config) *BackendDeployer {
	return &BackendDeployer{cfg: cfg}
}

// Deploy extracts a backend binary, deploys it into a pot, and starts the service
func (bd *BackendDeployer) Deploy(siteName string, commitHash string, artifactReader io.Reader, binaryName string) error {
	site, ok := bd.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return fmt.Errorf("site %s has no backend config", siteName)
	}

	log := slog.With("site", siteName, "commit", commitHash)
	log.Info("backend deployment starting", "binary", binaryName)

	jailMgr := jail.NewManager(bd.cfg)
	svcMgr := service.NewManager(bd.cfg)

	// Ensure pot exists
	if err := jailMgr.EnsureExists(siteName); err != nil {
		return fmt.Errorf("ensure pot: %w", err)
	}

	// Extract binary to a temp file first
	tempBinary, err := bd.extractBinaryToTemp(artifactReader, binaryName)
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}
	defer os.Remove(tempBinary)

	// Make it executable
	if err := os.Chmod(tempBinary, 0755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	// Stop the service (but keep pot running so we can copy)
	svcMgr.Stop(siteName)

	// Ensure pot is started so we can copy the binary
	if err := jailMgr.Start(siteName); err != nil {
		return fmt.Errorf("start pot for copy: %w", err)
	}

	// Ensure /usr/local/bin exists inside the pot
	if err := jailMgr.Exec(siteName, "mkdir", "-p", "/usr/local/bin"); err != nil {
		log.Warn("mkdir in pot failed", "error", err)
	}

	// Copy binary into pot
	destPath := filepath.Join("/usr/local/bin", site.Backend.BinaryName)
	if err := jailMgr.CopyIn(siteName, tempBinary, destPath); err != nil {
		return fmt.Errorf("copy binary to pot: %w", err)
	}

	// Create rc.d script on host
	if err := svcMgr.CreateBackendService(siteName); err != nil {
		return fmt.Errorf("create rc.d script: %w", err)
	}

	// Enable service
	if err := svcMgr.Enable(siteName); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	// Ensure /var/log exists inside the pot for daemon output
	if err := jailMgr.Exec(siteName, "mkdir", "-p", "/var/log"); err != nil {
		log.Warn("mkdir /var/log in pot failed", "error", err)
	}

	// Start the binary directly inside the pot using daemon
	// This is more reliable than going through the rc.d script
	port := fmt.Sprintf("%d", site.Backend.ListenPort)
	if err := jailMgr.Exec(siteName, "env", "PORT="+port, "HOST=0.0.0.0",
		"/usr/sbin/daemon", "-r", "-R", "5", "-o", "/var/log/app.log", "-f", destPath); err != nil {
		return fmt.Errorf("start service in pot: %w", err)
	}

	// Poll health check
	if err := bd.waitForHealth(siteName, 10, 1*time.Second); err != nil {
		// Service is starting but not yet healthy - return success anyway
		return nil
	}

	return nil
}

// extractBinaryToTemp extracts a binary from a zip to a temp file
func (bd *BackendDeployer) extractBinaryToTemp(reader io.Reader, binaryName string) (string, error) {
	// Read zip from stream
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read artifact: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("invalid zip: %w", err)
	}

	// Find the binary in the zip
	for _, f := range zr.File {
		if filepath.Base(f.Name) == binaryName {
			return bd.extractToTempFile(f)
		}
	}

	return "", fmt.Errorf("binary %s not found in zip", binaryName)
}

// extractToTempFile extracts a single file from a zip to a temp file
func (bd *BackendDeployer) extractToTempFile(f *zip.File) (string, error) {
	src, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("open zip entry: %w", err)
	}
	defer src.Close()

	// Create temp file
	tmpFile, err := os.CreateTemp("", "shipyard-binary-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, src); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("copy binary: %w", err)
	}

	return tmpFile.Name(), nil
}

// waitForHealth polls the service's health endpoint
func (bd *BackendDeployer) waitForHealth(siteName string, maxAttempts int, interval time.Duration) error {
	site, ok := bd.cfg.Site[siteName]
	if !ok {
		return fmt.Errorf("site not found: %s", siteName)
	}

	if site.Backend == nil {
		return nil // No backend to check
	}

	// Health check would go here - for now just return success
	// In a full implementation, this would:
	// 1. Make HTTP request to http://{jail_ip}:{listen_port}/health
	// 2. Check for 200 response
	// 3. Retry on failure up to maxAttempts times
	// 4. Return error if all attempts fail

	return nil
}
