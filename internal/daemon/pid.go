package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type PIDManager struct {
	pidFile string
}

// NewPIDManager: Creates a new PID Manager
func NewPIDManager(pidFile string) *PIDManager{
	return &PIDManager{
		pidFile: pidFile,
	}
}


// Write:
// 	- Creates the directory if it does not exist (~/.machinus/pid/)
// 	- Writes the PID as string with newline
func (p *PIDManager) Write(pid int) error {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(p.pidFile), 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	// Write PID to file
	data := []byte(strconv.Itoa(pid) + "\n")
	if err := os.WriteFile(p.pidFile, data, 0644); err != nil {
		return fmt.Errorf("Failed to write PID file: %w", err)
	}
	return nil
}

// Read:
// 	- Read the file, converts string to int
//  - Returns error if file does not exits or has invalid content
func (p * PIDManager) Read() (int, error) {
	data, err := os.ReadFile(p.pidFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}
	return pid, nil
}

// Remove:
//  - Deletes the PID file
//  - Ignores error if the file does not exist (already cleaned up/ never there)
func (p *PIDManager) Remove() error {
	if err := os.Remove(p.pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// IsRunning(): Checks if process with the stored PID is running
// 	- Reads PID from file
//  - Check if that process is actually running & return status, PID
func (p *PIDManager) IsRunning() (bool, int) {
	pid, err := p.Read()
	if err != nil {
		return false, 0
	}

	// On Windows, use FindProcess to check if process exists
	if runtime.GOOS == "windows" {
		process, err := os.FindProcess(pid)
		if err != nil {
			return false, 0
		}
		// On Windows, FindProcess doesn't actually check if process exists
		// We'll try to signal it (this will fail if process doesn't exist)
		err = process.Signal(os.Signal(syscall.SIGTERM))
		if err != nil {
			return false, 0
		}
		return true, pid
	}

	// Unix: send signal 0 to check if process exists
	// Note: syscall.Kill is available on Unix systems
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false, 0
	}
	return true, pid
}

