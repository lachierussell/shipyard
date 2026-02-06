package deploy

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/shipyard/shipyard/config"
)

// createTestZip creates a zip file with the given files
func createTestZip(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write zip entry %s: %v", name, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	return buf
}

func TestExtractZip_BasicExtraction(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "extract")

	files := map[string]string{
		"index.html":     "<html>Hello</html>",
		"css/style.css":  "body { color: red; }",
		"js/app.js":      "console.log('hello');",
		"assets/logo.png": "fake-png-content",
	}

	zipBuf := createTestZip(t, files)

	cfg := &config.Config{}
	deployer := NewFrontendDeployer(cfg)

	err := deployer.extractZip(bytes.NewReader(zipBuf.Bytes()), targetDir)
	if err != nil {
		t.Fatalf("extractZip() error = %v", err)
	}

	// Verify files exist
	for name, expectedContent := range files {
		path := filepath.Join(targetDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("File %s not found: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content = %q, want %q", name, string(content), expectedContent)
		}
	}
}

func TestExtractZip_ZipSlipPrevention_DotDot(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "extract")

	// Create a malicious zip with path traversal
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// Try to escape with ../
	f, _ := w.Create("../../../etc/passwd")
	f.Write([]byte("malicious content"))
	w.Close()

	cfg := &config.Config{}
	deployer := NewFrontendDeployer(cfg)

	err := deployer.extractZip(bytes.NewReader(buf.Bytes()), targetDir)
	if err == nil {
		t.Error("extractZip() should reject path traversal attempts")
	}

	if err != nil && !contains(err.Error(), "zip slip") {
		t.Errorf("extractZip() error should mention zip slip, got: %v", err)
	}
}

func TestExtractZip_ZipSlipPrevention_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "extract")

	// Create a malicious zip with absolute path
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, _ := w.Create("/etc/passwd")
	f.Write([]byte("malicious content"))
	w.Close()

	cfg := &config.Config{}
	deployer := NewFrontendDeployer(cfg)

	err := deployer.extractZip(bytes.NewReader(buf.Bytes()), targetDir)
	if err == nil {
		t.Error("extractZip() should reject absolute path attempts")
	}
}

func TestExtractZip_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "extract")

	files := map[string]string{
		"deep/nested/path/file.txt": "content",
	}

	zipBuf := createTestZip(t, files)

	cfg := &config.Config{}
	deployer := NewFrontendDeployer(cfg)

	err := deployer.extractZip(bytes.NewReader(zipBuf.Bytes()), targetDir)
	if err != nil {
		t.Fatalf("extractZip() error = %v", err)
	}

	// Verify nested directory was created
	expectedPath := filepath.Join(targetDir, "deep", "nested", "path", "file.txt")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("extractZip() should create nested directories")
	}
}

func TestExtractZip_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "extract")

	cfg := &config.Config{}
	deployer := NewFrontendDeployer(cfg)

	// Pass invalid zip content
	err := deployer.extractZip(bytes.NewReader([]byte("not a zip file")), targetDir)
	if err == nil {
		t.Error("extractZip() should fail for invalid zip content")
	}
}

func TestUpdateLatestSymlink(t *testing.T) {
	dir := t.TempDir()

	// Create commit directories
	commit1 := "abc1234"
	commit2 := "def5678"
	os.MkdirAll(filepath.Join(dir, commit1), 0755)
	os.MkdirAll(filepath.Join(dir, commit2), 0755)

	cfg := &config.Config{}
	deployer := NewFrontendDeployer(cfg)

	// Create first symlink
	err := deployer.updateLatestSymlink(dir, commit1)
	if err != nil {
		t.Fatalf("updateLatestSymlink() error = %v", err)
	}

	// Verify symlink points to commit1
	target, err := os.Readlink(filepath.Join(dir, "latest"))
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != commit1 {
		t.Errorf("Symlink target = %q, want %q", target, commit1)
	}

	// Update to commit2
	err = deployer.updateLatestSymlink(dir, commit2)
	if err != nil {
		t.Fatalf("updateLatestSymlink() second call error = %v", err)
	}

	// Verify symlink now points to commit2
	target, err = os.Readlink(filepath.Join(dir, "latest"))
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != commit2 {
		t.Errorf("Symlink target = %q, want %q", target, commit2)
	}
}

func TestDeploy_SiteNotFound(t *testing.T) {
	cfg := &config.Config{
		Site: map[string]config.SiteConfig{},
	}
	deployer := NewFrontendDeployer(cfg)

	_, _, err := deployer.Deploy("nonexistent", "abc1234", nil, "", false)
	if err == nil {
		t.Error("Deploy() should fail for nonexistent site")
	}
}

// Helper function
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
