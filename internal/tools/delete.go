package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinus/cloud-agent/internal/types"
)

// DeleteTool deletes files and directories
type DeleteTool struct {
	confirmRequired bool
}

// NewDeleteTool creates a new delete tool
func NewDeleteTool(requireConfirm bool) *DeleteTool {
	return &DeleteTool{
		confirmRequired: requireConfirm,
	}
}

func (t *DeleteTool) Name() string {
	return "delete"
}

func (t *DeleteTool) Description() string {
	return "Delete files and directories. Supports recursive deletion of directories. Use with caution - deleted files cannot be recovered."
}

func (t *DeleteTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"path": "temp.txt",
			},
			Description: "Delete a single file",
		},
		{
			Input: map[string]any{
				"path": "old_folder",
				"recursive": true,
			},
			Description: "Delete directory and all contents",
		},
		{
			Input: map[string]any{
				"path": "*.log",
			},
			Description: "Delete files matching pattern (wildcard not supported - use glob first)",
		},
	}
}

func (t *DeleteTool) WhenToUse() string {
	return "Use to remove files or directories that are no longer needed. Always verify paths before deletion as this operation cannot be undone."
}

func (t *DeleteTool) ChainsWith() []string {
	return []string{"glob", "list", "copy"}
}

func (t *DeleteTool) ValidateArgs(args map[string]any) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("missing or invalid 'path' argument")
	}

	// Security: Prevent wildcard deletions
	if strings.Contains(path, "*") || strings.Contains(path, "?") {
		return fmt.Errorf("wildcards not supported - use glob tool to find files first")
	}

	// Security: Prevent deleting current directory
	if path == "." || path == "./" {
		return fmt.Errorf("cannot delete current directory")
	}

	return nil
}

func (t *DeleteTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, _ := args["path"].(string)
	recursive, _ := args["recursive"].(bool)

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("path does not exist: %s", path),
			}, nil
		}
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to access path: %v", err),
		}, nil
	}

	// Count items before deletion
	itemCount := 0
	if info.IsDir() {
		itemCount = t.countItems(path, recursive)
	} else {
		itemCount = 1
	}

	// Perform deletion
	if info.IsDir() {
		if !recursive {
			// Check if directory is empty
			entries, err := os.ReadDir(path)
			if err == nil && len(entries) > 0 {
				return types.ToolResult{
					Success: false,
					Error:   fmt.Sprintf("directory is not empty (contains %d items) - use recursive=true to delete", len(entries)),
				}, nil
			}
		}

		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("deletion failed: %v", err),
		}, nil
	}

	itemType := "file"
	if info.IsDir() {
		itemType = "directory"
	}

	output := fmt.Sprintf("Deleted %s: %s\n", itemType, path)
	if itemCount > 1 {
		output += fmt.Sprintf("Total items removed: %d", itemCount)
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"path":  path,
			"type":  itemType,
			"count": itemCount,
		},
	}, nil
}

func (t *DeleteTool) countItems(path string, recursive bool) int {
	count := 0

	info, err := os.Stat(path)
	if err != nil {
		return count
	}

	if !info.IsDir() {
		return 1
	}

	if !recursive {
		entries, err := os.ReadDir(path)
		if err != nil {
			return 0
		}
		return len(entries) + 1 // +1 for the directory itself
	}

	// Recursive count
	filepath.Walk(path, func(subPath string, info os.FileInfo, err error) error {
		if err == nil {
			count++
		}
		return nil
	})

	return count
}
