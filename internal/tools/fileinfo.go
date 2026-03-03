package tools

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// FileInfoTool provides detailed file information
type FileInfoTool struct{}

// NewFileInfoTool creates a new file info tool
func NewFileInfoTool() *FileInfoTool {
	return &FileInfoTool{}
}

func (t *FileInfoTool) Name() string {
	return "fileinfo"
}

func (t *FileInfoTool) Description() string {
	return "Get detailed information about a file or directory including size, permissions, timestamps, and MIME type."
}

func (t *FileInfoTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"path": "document.pdf",
			},
			Description: "Get detailed information about a file",
		},
		{
			Input: map[string]any{
				"path": "src/",
			},
			Description: "Get information about a directory",
		},
		{
			Input: map[string]any{
				"path": "image.png",
				"include_mime": true,
			},
			Description: "Get file info with MIME type detection",
		},
	}
}

func (t *FileInfoTool) WhenToUse() string {
	return "Use to get comprehensive information about files and directories - size, permissions, creation/modification times, file type, and MIME type."
}

func (t *FileInfoTool) ChainsWith() []string {
	return []string{"list", "read_file", "glob"}
}

func (t *FileInfoTool) ValidateArgs(args map[string]any) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("missing or invalid 'path' argument")
	}

	return nil
}

func (t *FileInfoTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, _ := args["path"].(string)
	includeMime, _ := args["include_mime"].(bool)

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("file not found: %s", path),
			}, nil
		}
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to access file: %v", err),
		}, nil
	}

	// Build result
	output := fmt.Sprintf("File Information: %s\n", path)
	output += fmt.Sprintf("%s\n", "─"+strings.Repeat("─", 50))

	// Type
	fileType := "file"
	if info.IsDir() {
		fileType = "directory"
	} else if info.Mode()&fs.ModeSymlink != 0 {
		fileType = "symlink"
	}
	output += fmt.Sprintf("Type:        %s\n", fileType)

	// Size
	if !info.IsDir() {
		size := info.Size()
		output += fmt.Sprintf("Size:        %s (%d bytes)\n", t.formatSize(size), size)
	}

	// Permissions
	output += fmt.Sprintf("Permissions: %s\n", info.Mode().String())

	// Owner (Unix only)
	if uid, gid, err := t.getOwner(path); err == nil {
		output += fmt.Sprintf("Owner:       UID=%d, GID=%d\n", uid, gid)
	}

	// Timestamps
	output += fmt.Sprintf("Modified:    %s\n", info.ModTime().Format(time.RFC3339))
	if accessTime := t.getAccessTime(path); !accessTime.IsZero() {
		output += fmt.Sprintf("Accessed:    %s\n", accessTime.Format(time.RFC3339))
	}

	// MIME type (for files only)
	if includeMime && !info.IsDir() {
		mime := t.detectMIMEType(path)
		output += fmt.Sprintf("MIME Type:   %s\n", mime)
	}

	// Absolute path
	if absPath, err := filepath.Abs(path); err == nil {
		output += fmt.Sprintf("Path:        %s\n", absPath)
	}

	// Create data map
	data := map[string]any{
		"name":        filepath.Base(path),
		"path":        path,
		"type":        fileType,
		"size":        info.Size(),
		"permissions": info.Mode().String(),
		"modified":    info.ModTime().Format(time.RFC3339),
		"is_dir":      info.IsDir(),
	}

	if includeMime && !info.IsDir() {
		data["mime_type"] = t.detectMIMEType(path)
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data:    data,
	}, nil
}

func (t *FileInfoTool) formatSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/float64(TB))
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func (t *FileInfoTool) detectMIMEType(path string) string {
	// Read first 512 bytes for MIME detection
	file, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	if n == 0 {
		return "text/plain"
	}

	// Detect MIME type
	mimeType := http.DetectContentType(buffer[:n])

	// Refine some common types
	ext := filepath.Ext(path)
	switch ext {
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".md":
		return "text/markdown"
	case ".go":
		return "text/x-go"
	case ".js":
		return "text/javascript"
	case ".ts":
		return "text/typescript"
	case ".tsx", ".jsx":
		return "text/javascript"
	}

	return mimeType
}

func (t *FileInfoTool) getAccessTime(path string) time.Time {
	// Try to get access time (platform-specific)
	if info, err := os.Stat(path); err == nil {
		// On Unix, this requires syscalls
		// For now, return mod time as fallback
		return info.ModTime()
	}
	return time.Time{}
}

func (t *FileInfoTool) getOwner(path string) (int, int, error) {
	// Platform-specific owner information
	// On Windows, this doesn't apply
	// On Unix, would require syscalls
	return 0, 0, fmt.Errorf("not implemented")
}
