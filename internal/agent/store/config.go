// Package store provides filesystem-based session persistence.
package store

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultRelayDir returns the default relay directory for storing sessions.
// C:/Users/<Username>/.relay/sessions on Windows
// /home/<username>/.relay/sessions on Unix
func DefaultRelayDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}

	var basePath string
	if runtime.GOOS == "windows" {
		basePath = filepath.Join(homeDir, ".relay")
	} else {
		basePath = filepath.Join(homeDir, ".relay")
	}

	return filepath.Join(basePath, "sessions")
}

// EnsureRelayDir creates the relay sessions directory if it doesn't exist.
func EnsureRelayDir() (string, error) {
	dir := DefaultRelayDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
