// Package store provides filesystem-based session persistence.
package store

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// SanitizePath converts a file path to a safe directory name.
// D:\MVP\the-machinus-project → D-MVP-the-machinus-project
func SanitizePath(path string) string {
	// Convert to forward slashes for consistent handling
	path = filepath.ToSlash(path)

	// Remove drive letter on Windows (D:/) and replace with dash
	if runtime.GOOS == "windows" {
		if len(path) >= 2 && path[1] == ':' {
			path = path[:1] + "-" + path[2:]
		}
	}

	// Replace slashes with dashes
	path = strings.ReplaceAll(path, "/", "-")

	// Remove any characters that aren't safe for directory names
	// Keep alphanumeric, dash, underscore, dot
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)
	path = re.ReplaceAllString(path, "-")

	// Collapse multiple consecutive dashes
	re = regexp.MustCompile(`-+`)
	path = re.ReplaceAllString(path, "-")

	// Trim dashes from start and end
	path = strings.Trim(path, "-")

	// Handle empty result
	if path == "" {
		path = "unnamed"
	}

	return path
}

// SessionDir returns the session directory path for a given working directory and session ID.
// <rootDir>/<sanitizedWorkDir>/session-<sessionID>
func SessionDir(rootDir, workDir, sessionID string) string {
	sanitized := SanitizePath(workDir)
	return filepath.Join(rootDir, sanitized, "session-"+sessionID)
}
