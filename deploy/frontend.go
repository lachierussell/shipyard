package deploy

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lachierussell/shipyard/config"
	"github.com/lachierussell/shipyard/nginx"
	"github.com/lachierussell/shipyard/ssl"
)

// FrontendDeployer handles frontend deployment
type FrontendDeployer struct {
	cfg *config.Config
}

// NewFrontendDeployer creates a new frontend deployer
func NewFrontendDeployer(cfg *config.Config) *FrontendDeployer {
	return &FrontendDeployer{cfg: cfg}
}

// Deploy extracts a frontend zip, optionally updates the symlink, and deploys the nginx config.
// Set updateLatest to true for main branch deployments, false for branch previews.
func (fd *FrontendDeployer) Deploy(siteName string, commitHash string, artifactReader io.Reader, nginxConfig string, updateLatest bool) (bool, string, error) {
	log := slog.With("site", siteName, "commit", commitHash)

	site, ok := fd.cfg.Site[siteName]
	if !ok {
		return false, "", fmt.Errorf("site not found: %s", siteName)
	}

	log.Info("frontend deployment starting", "update_latest", updateLatest)

	// Create the commit directory
	commitDir := filepath.Join(site.FrontendRoot, commitHash)
	if err := os.MkdirAll(commitDir, 0755); err != nil {
		return false, "", fmt.Errorf("mkdir commit dir: %w", err)
	}

	// Extract zip into commit directory
	if err := fd.extractZip(artifactReader, commitDir); err != nil {
		return false, "", fmt.Errorf("extract zip: %w", err)
	}

	// Write default robots.txt if not present
	robotsPath := filepath.Join(commitDir, "robots.txt")
	if _, err := os.Stat(robotsPath); os.IsNotExist(err) {
		if err := os.WriteFile(robotsPath, []byte(nginx.GenerateRobotsTxt()), 0644); err != nil {
			return false, "", fmt.Errorf("write robots.txt: %w", err)
		}
	}

	// Atomically update the latest symlink (only for main branch deployments)
	if updateLatest {
		if err := fd.updateLatestSymlink(site.FrontendRoot, commitHash); err != nil {
			return false, "", fmt.Errorf("update symlink: %w", err)
		}
	}

	// Deploy nginx config (validate + reload)
	// If site has a backend, use combined template to preserve backend proxy
	nginxMgr := nginx.NewManager(fd.cfg)
	var reloaded bool
	var errMsg string
	var err error

	if site.Backend != nil {
		// Generate combined frontend+backend config
		var combinedConfig string
		if site.SSLEnabled {
			certPath, keyPath := ssl.CertPaths(siteName)
			combinedConfig = nginx.GenerateSiteCombinedConfigHTTPS(siteName, site.FrontendRoot, site.Backend.ListenPort, site.Backend.ProxyPath, certPath, keyPath)
		} else {
			combinedConfig = nginx.GenerateSiteCombinedConfig(siteName, site.FrontendRoot, site.Backend.ListenPort, site.Backend.ProxyPath)
		}
		reloaded, errMsg, err = nginxMgr.DeploySiteConfigRaw(siteName, combinedConfig)
	} else {
		// Frontend-only site, use provided config
		reloaded, errMsg, err = nginxMgr.DeploySiteConfig(siteName, nginxConfig)
	}

	if err != nil {
		return false, "", err
	}

	// If nginx validation failed, return partial success (frontend is live, nginx config wasn't applied)
	if !reloaded {
		return false, errMsg, nil
	}

	return true, "", nil
}

// extractZip extracts a zip file into a target directory with zip-slip protection
func (fd *FrontendDeployer) extractZip(reader io.Reader, targetDir string) error {
	// Read the zip into memory (it's coming from a multipart upload, so it's a stream)
	// We need to seek, so convert to bytes first
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid zip: %w", err)
	}

	for _, f := range zr.File {
		if err := fd.extractZipEntry(f, targetDir); err != nil {
			return err
		}
	}

	return nil
}

// extractZipEntry extracts a single zip entry with zip-slip protection
func (fd *FrontendDeployer) extractZipEntry(f *zip.File, targetDir string) error {
	// Sanitize path: clean it and reject if it tries to escape
	cleanPath := filepath.Clean(f.Name)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return fmt.Errorf("zip slip detected: %s", f.Name)
	}

	// Full target path
	fullPath := filepath.Join(targetDir, cleanPath)

	// Ensure it's still within targetDir
	rel, err := filepath.Rel(targetDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("zip slip detected: %s", f.Name)
	}

	// Create directories
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
		return nil
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	// Extract file
	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("extract file: %w", err)
	}

	return nil
}

// updateLatestSymlink atomically updates the "latest" symlink to point to the new commit
// It auto-detects if content is in a subdirectory like "dist/" and points there instead
func (fd *FrontendDeployer) updateLatestSymlink(frontendRoot string, commitHash string) error {
	latestPath := filepath.Join(frontendRoot, "latest")
	tmpPath := filepath.Join(frontendRoot, "latest.tmp")

	// Remove temp symlink if it exists from a failed previous attempt
	os.Remove(tmpPath)

	// Check if latest exists and is not a symlink (e.g., a directory from initial install)
	if info, err := os.Lstat(latestPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			// It's a regular file or directory, remove it
			if err := os.RemoveAll(latestPath); err != nil {
				return fmt.Errorf("remove existing latest: %w", err)
			}
		}
	}

	// Determine the actual content directory
	// If the commit folder has a single subdirectory with index.html, use that
	symlinkTarget := commitHash
	commitDir := filepath.Join(frontendRoot, commitHash)

	// Check common build output directories
	for _, subdir := range []string{"dist", "build", "out", "public"} {
		subdirPath := filepath.Join(commitDir, subdir)
		indexPath := filepath.Join(subdirPath, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			// Found index.html in subdirectory
			symlinkTarget = filepath.Join(commitHash, subdir)
			break
		}
	}

	// Create symlink to the content directory
	if err := os.Symlink(symlinkTarget, tmpPath); err != nil {
		return fmt.Errorf("create temp symlink: %w", err)
	}

	// Atomic rename: tmpPath -> latestPath
	if err := os.Rename(tmpPath, latestPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename symlink: %w", err)
	}

	return nil
}
