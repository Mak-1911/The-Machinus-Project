package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/machinus/cloud-agent/internal/types"
)

// CopyTool copies files and directories
type CopyTool struct {
	maxSize int64
}

// NewCopyTool creates a new copy tool
func NewCopyTool(maxSizeGB int) *CopyTool {
	maxSize := int64(maxSizeGB) * 1024 * 1024 * 1024
	return &CopyTool{
		maxSize: maxSize,
	}
}

func (t *CopyTool) Name() string {
	return "copy"
}

func (t *CopyTool) Description() string {
	return "Copy files and directories. Supports recursive directory copying, preserving permissions and timestamps."
}

func (t *CopyTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"src": "file.txt",
				"dest": "backup.txt",
			},
			Description: "Copy a single file",
		},
		{
			Input: map[string]any{
				"src": "src/",
				"dest": "backup/",
			},
			Description: "Copy entire directory recursively",
		},
		{
			Input: map[string]any{
				"src": "data.txt",
				"dest": "existing.txt",
				"overwrite": true,
			},
			Description: "Copy file and overwrite if destination exists",
		},
	}
}

func (t *CopyTool) WhenToUse() string {
	return "Use to duplicate files, backup directories, or copy files to new locations. Preserves file metadata and supports recursive directory copying."
}

func (t *CopyTool) ChainsWith() []string {
	return []string{"read_file", "write_file", "glob", "list"}
}

func (t *CopyTool) ValidateArgs(args map[string]any) error {
	src, ok := args["src"].(string)
	if !ok || src == "" {
		return fmt.Errorf("missing or invalid 'src' argument")
	}

	dest, ok := args["dest"].(string)
	if !ok || dest == "" {
		return fmt.Errorf("missing or invalid 'dest' argument")
	}

	return nil
}

func (t *CopyTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	src, _ := args["src"].(string)
	dest, _ := args["dest"].(string)
	overwrite, _ := args["overwrite"].(bool)

	// Get source info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot access source: %v", err),
		}, nil
	}

	// Check if destination exists
	if _, err := os.Stat(dest); err == nil {
		if !overwrite {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("destination already exists: %s (use overwrite=true to overwrite)", dest),
			}, nil
		}
	}

	// Perform copy based on source type
	var copied int
	if srcInfo.IsDir() {
		copied, err = t.copyDir(src, dest, overwrite)
	} else {
		copied, err = t.copyFile(src, dest, overwrite)
	}

	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("copy failed: %v", err),
		}, nil
	}

	output := fmt.Sprintf("Successfully copied %s -> %s\n", src, dest)
	if copied > 1 {
		output += fmt.Sprintf("Total items copied: %d", copied)
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"src":      src,
			"dest":     dest,
			"items":    copied,
			"size":     srcInfo.Size(),
		},
	}, nil
}

func (t *CopyTool) copyFile(src, dest string, overwrite bool) (int, error) {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	// Get file info
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return 0, err
	}

	// Check size limit
	if srcInfo.Size() > t.maxSize {
		return 0, fmt.Errorf("file too large (%d bytes, max %d bytes)", srcInfo.Size(), t.maxSize)
	}

	// Create destination directory if needed
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return 0, err
	}

	// Create destination file
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return 0, err
	}
	defer destFile.Close()

	// Copy content
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return 0, err
	}

	// Preserve timestamps
	if err := os.Chtimes(dest, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return 0, err
	}

	return 1, nil
}

func (t *CopyTool) copyDir(src, dest string, overwrite bool) (int, error) {
	// Create destination directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(dest, srcInfo.Mode()); err != nil {
		return 0, err
	}

	// Read directory contents
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			subCount, err := t.copyDir(srcPath, destPath, overwrite)
			if err != nil {
				return count, err
			}
			count += subCount
		} else {
			// Copy file
			_, err := t.copyFile(srcPath, destPath, overwrite)
			if err != nil {
				return count, err
			}
			count++
		}
	}

	return count, nil
}
