package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/machinus/cloud-agent/internal/types"
)

// MoveTool moves and renames files and directories
type MoveTool struct{}

// NewMoveTool creates a new move tool
func NewMoveTool() *MoveTool {
	return &MoveTool{}
}

func (t *MoveTool) Name() string {
	return "move"
}

func (t *MoveTool) Description() string {
	return "Move and rename files and directories. Supports cross-device moves (copy + delete)."
}

func (t *MoveTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"src": "file.txt",
				"dest": "renamed.txt",
			},
			Description: "Rename a file in the same directory",
		},
		{
			Input: map[string]any{
				"src": "old_folder/file.txt",
				"dest": "new_folder/file.txt",
			},
			Description: "Move file to different directory",
		},
		{
			Input: map[string]any{
				"src": "source_dir/",
				"dest": "target_dir/",
			},
			Description: "Move entire directory",
		},
	}
}

func (t *MoveTool) WhenToUse() string {
	return "Use to move files between directories, rename files, or reorganize directory structures. Works across different drives/devices."
}

func (t *MoveTool) ChainsWith() []string {
	return []string{"copy", "glob", "list", "delete"}
}

func (t *MoveTool) ValidateArgs(args map[string]any) error {
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

func (t *MoveTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	src, _ := args["src"].(string)
	dest, _ := args["dest"].(string)
	overwrite, _ := args["overwrite"].(bool)

	// Check source exists
	srcInfo, err := os.Stat(src)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("source not found: %v", err),
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
		// Remove destination if overwriting
		if err := os.RemoveAll(dest); err != nil {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to remove existing destination: %v", err),
			}, nil
		}
	}

	// Create destination directory if needed
	destDir := filepath.Dir(dest)
	if destDir != "" && destDir != "." {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to create destination directory: %v", err),
			}, nil
		}
	}

	// Try simple rename first (same device)
	err = os.Rename(src, dest)
	if err == nil {
		return types.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Moved %s -> %s", src, dest),
			Data: map[string]any{
				"src":      src,
				"dest":     dest,
				"type":     srcInfo.Mode().String(),
			},
		}, nil
	}

	// If rename failed, try copy + delete (cross-device move)
	if srcInfo.IsDir() {
		return t.moveDir(src, dest)
	}
	return t.moveFile(src, dest)
}

func (t *MoveTool) moveFile(src, dest string) (types.ToolResult, error) {
	// Open source
	srcFile, err := os.Open(src)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to open source: %v", err),
		}, nil
	}
	defer srcFile.Close()

	// Get source info
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get source info: %v", err),
		}, nil
	}

	// Create destination
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create destination: %v", err),
		}, nil
	}
	defer destFile.Close()

	// Copy content
	if _, err := destFile.ReadFrom(srcFile); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("copy failed: %v", err),
		}, nil
	}

	// Preserve timestamps
	if err := os.Chtimes(dest, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to preserve timestamps: %v", err),
		}, nil
	}

	// Remove source
	if err := os.Remove(src); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("copied but failed to remove source: %v", err),
		}, nil
	}

	return types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Moved %s -> %s", src, dest),
		Data: map[string]any{
			"src":  src,
			"dest": dest,
			"size": srcInfo.Size(),
		},
	}, nil
}

func (t *MoveTool) moveDir(src, dest string) (types.ToolResult, error) {
	// Use copy tool logic to copy directory
	copyTool := NewCopyTool(100) // 100GB limit

	copyResult, err := copyTool.Execute(nil, map[string]any{
		"src":       src,
		"dest":      dest,
		"overwrite": true,
	})
	if err != nil || !copyResult.Success {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to copy directory: %v", err),
		}, nil
	}

	// Remove source directory
	if err := os.RemoveAll(src); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("copied but failed to remove source directory: %v", err),
		}, nil
	}

	return types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Moved %s -> %s", src, dest),
		Data: map[string]any{
			"src":  src,
			"dest": dest,
		},
	}, nil
}
