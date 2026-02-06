package update

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Updater handles atomic self-updates of the shipyard binary
type Updater struct {
	binaryPath string
}

// NewUpdater creates a new Updater for the given binary path
func NewUpdater(binaryPath string) *Updater {
	return &Updater{binaryPath: binaryPath}
}

// Update performs an atomic update of the binary from the given reader.
// The update flow:
// 1. Write incoming binary to {binaryPath}.new
// 2. Set executable permissions (0755)
// 3. Validate by running {binaryPath}.new version
// 4. Rename current binary to {binaryPath}.old (backup)
// 5. Rename .new to main path (atomic!)
func (u *Updater) Update(newBinary io.Reader) error {
	newPath := u.binaryPath + ".new"
	oldPath := u.binaryPath + ".old"

	// Step 1: Write new binary to temp location
	if err := u.writeNewBinary(newPath, newBinary); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}

	// Step 2: Validate the new binary
	if err := u.validateBinary(newPath); err != nil {
		os.Remove(newPath)
		return fmt.Errorf("validate binary: %w", err)
	}

	// Step 3: Atomic replacement
	if err := u.atomicReplace(newPath, oldPath); err != nil {
		os.Remove(newPath)
		return fmt.Errorf("atomic replace: %w", err)
	}

	return nil
}

// writeNewBinary writes the binary data to the specified path with executable permissions
func (u *Updater) writeNewBinary(path string, data io.Reader) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Create the file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	// Copy data
	if _, err := io.Copy(f, data); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	// Ensure data is flushed to disk
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}

	return nil
}

// validateBinary checks that the binary at path is executable and responds to "version"
func (u *Updater) validateBinary(path string) error {
	// Check file exists and has executable bit
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat binary: %w", err)
	}

	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable")
	}

	// Run the binary with "version" command to verify it works
	cmd := exec.Command(path, "version")
	cmd.Env = os.Environ()

	// Set a timeout for validation
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("binary validation failed: %w", err)
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		return fmt.Errorf("binary validation timed out")
	}

	return nil
}

// atomicReplace performs the atomic replacement of the binary.
// It renames the current binary to .old, then renames .new to the main path.
// If the final rename fails, it attempts to restore the old binary.
func (u *Updater) atomicReplace(newPath, oldPath string) error {
	// Remove any existing .old backup (from previous update)
	os.Remove(oldPath)

	// Rename current binary to .old (backup)
	if err := os.Rename(u.binaryPath, oldPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Rename .new to main path (this is the atomic operation)
	if err := os.Rename(newPath, u.binaryPath); err != nil {
		// Try to restore the old binary
		if restoreErr := os.Rename(oldPath, u.binaryPath); restoreErr != nil {
			return fmt.Errorf("rename failed (%w) and restore failed (%w)", err, restoreErr)
		}
		return fmt.Errorf("rename new binary: %w", err)
	}

	return nil
}

// HasBackup returns true if a backup (.old) binary exists
func (u *Updater) HasBackup() bool {
	oldPath := u.binaryPath + ".old"
	_, err := os.Stat(oldPath)
	return err == nil
}

// Rollback restores the previous binary from the .old backup
func (u *Updater) Rollback() error {
	oldPath := u.binaryPath + ".old"

	// Check backup exists
	if _, err := os.Stat(oldPath); err != nil {
		return fmt.Errorf("no backup found at %s", oldPath)
	}

	// Move current to .new (in case we need to undo)
	newPath := u.binaryPath + ".new"
	os.Remove(newPath) // Remove any existing .new

	if err := os.Rename(u.binaryPath, newPath); err != nil {
		return fmt.Errorf("move current binary: %w", err)
	}

	// Move .old to main path
	if err := os.Rename(oldPath, u.binaryPath); err != nil {
		// Try to restore
		os.Rename(newPath, u.binaryPath)
		return fmt.Errorf("restore backup: %w", err)
	}

	// Clean up the .new file (the binary we just rolled back from)
	os.Remove(newPath)

	return nil
}
