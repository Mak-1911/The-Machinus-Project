// Package app provides the agent coordinator for LLM integration.
package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/machinus/cloud-agent/internal/agent/store"
)

// SetupSessionPersistence configures filesystem-based session persistence
// for the agent coordinator. It returns the store and history provider
// for further use if needed.
func SetupSessionPersistence(ctx context.Context, workDir string) (*store.FilesystemStore, *store.FileHistoryProvider, error) {
	// Get/create the relay sessions directory
	rootDir, err := store.EnsureRelayDir()
	if err != nil {
		return nil, nil, err
	}

	// Create the filesystem store
	fsStore := store.NewFilesystemStore(rootDir)

	// Create the history provider
	historyProvider := store.NewFileHistoryProvider(fsStore)

	return fsStore, historyProvider, nil
}

// SetupSessionPersistenceWithPath configures persistence with a custom root path.
func SetupSessionPersistenceWithPath(rootDir, workDir string) (*store.FilesystemStore, *store.FileHistoryProvider, error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, nil, err
	}

	fsStore := store.NewFilesystemStore(rootDir)
	historyProvider := store.NewFileHistoryProvider(fsStore)

	return fsStore, historyProvider, nil
}

// GetWorkingDir returns the current working directory.
func GetWorkingDir() (string, error) {
	return os.Getwd()
}

// GetSessionsRootDir returns the root directory for session storage.
func GetSessionsRootDir() string {
	return filepath.Join(store.DefaultRelayDir())
}
