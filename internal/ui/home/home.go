// Package home provides home directory utilities for the UI.
package home

import (
	"os"
	"path/filepath"
	"strings"
)

// Short returns a shortened path with ~ for home directory.
func Short(path string) string {
	if path == "" {
		return ""
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	// Handle both forward and backslashes
	homeDirSlash := filepath.ToSlash(homeDir)
	pathSlash := filepath.ToSlash(path)

	if strings.HasPrefix(pathSlash, homeDirSlash) {
		if len(path) > len(homeDir) {
			sep := "~"
			if len(homeDir) < len(path) && (path[len(homeDir)] == '\\' || path[len(homeDir)] == '/') {
				sep = ""
			}
			return "~" + sep + path[len(homeDir):]
		}
		return "~"
	}

	return path
}

// Expand expands ~ to the home directory.
func Expand(path string) string {
	if path == "" || !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return homeDir
	}

	// Handle ~/path
	if len(path) > 1 && (path[1] == '/' || path[1] == '\\') {
		return filepath.Join(homeDir, path[2:])
	}

	return filepath.Join(homeDir, path[1:])
}

// Dir returns the home directory.
func Dir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return homeDir
}
