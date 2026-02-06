package pidfile

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// File manages a lock-based PID file for single-instance enforcement
type File struct {
	path string
	file *os.File
}

// Create creates and locks a PID file. Returns error if already locked by another process.
func Create(path string) (*File, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("create pidfile: %w", err)
	}

	// Try to acquire exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("pidfile locked (another instance running): %w", err)
	}

	// Write our PID
	pid := os.Getpid()
	if _, err := f.WriteString(strconv.Itoa(pid) + "\n"); err != nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		return nil, fmt.Errorf("write pid: %w", err)
	}

	return &File{path: path, file: f}, nil
}

// Close releases the lock and removes the PID file
func (pf *File) Close() error {
	if pf.file == nil {
		return nil
	}
	syscall.Flock(int(pf.file.Fd()), syscall.LOCK_UN)
	pf.file.Close()
	os.Remove(pf.path)
	return nil
}
