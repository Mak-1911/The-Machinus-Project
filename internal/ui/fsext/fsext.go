// Package fsext provides file system extensions for the UI.
package fsext

import (
	"os"
	"path/filepath"
	"strings"
)

// PrettyPath returns a shortened, user-friendly path.
// It replaces the home directory with "~" and uses forward slashes.
func PrettyPath(path string) string {
	if path == "" {
		return ""
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// Replace home directory with ~
		if strings.HasPrefix(path, homeDir) {
			path = "~" + path[len(homeDir):]
		}
	}

	// Convert backslashes to forward slashes on Windows
	path = filepath.ToSlash(path)

	return path
}

// BaseName returns the base name of a path.
func BaseName(path string) string {
	return filepath.Base(path)
}

// DirName returns the directory name of a path.
func DirName(path string) string {
	return filepath.Dir(path)
}

// JoinPath joins path elements.
func JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// IsAbs checks if a path is absolute.
func IsAbs(path string) bool {
	return filepath.IsAbs(path)
}

// Clean cleans a path.
func Clean(path string) string {
	return filepath.Clean(path)
}

// ListDirectory lists files in a directory.
func ListDirectory(dir string, ignore []string, depth, limit int) ([]string, int, error) {
	// Simple implementation - list immediate files only
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		// Skip ignored files/directories
		if ignore != nil {
			ignored := false
			for _, pattern := range ignore {
				if strings.Contains(name, pattern) {
					ignored = true
					break
				}
			}
			if ignored {
				continue
			}
		}

		fullPath := filepath.Join(dir, name)
		if entry.IsDir() {
			fullPath += "/"
		}
		files = append(files, fullPath)

		if limit > 0 && len(files) >= limit {
			break
		}
	}

	return files, len(files), nil
}

// DirTrim trims a directory path to a maximum length.
func DirTrim(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	if maxLen < 4 {
		return "..."
	}
	// Keep the beginning and end of the path
	halfLen := (maxLen - 3) / 2
	return path[:halfLen] + "..." + path[len(path)-halfLen:]
}

// ParsePastedFiles parses pasted text into file paths.
func ParsePastedFiles(text string) []string {
	// Placeholder - return empty slice
	return nil
}
