package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/machinus/cloud-agent/internal/types"
)

// MakeDirectoryTool creates directories
type MakeDirectoryTool struct{}

// NewMakeDirectoryTool creates a new make directory tool
func NewMakeDirectoryTool() *MakeDirectoryTool {
	return &MakeDirectoryTool{}
}

func (t *MakeDirectoryTool) Name() string {
	return "mkdir"
}

func (t *MakeDirectoryTool) Description() string {
	return "Create directories. Supports creating nested directories in one operation (like mkdir -p)."
}

func (t *MakeDirectoryTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"path": "new_folder",
			},
			Description: "Create a single directory",
		},
		{
			Input: map[string]any{
				"path": "parent/child/grandchild",
				"parents": true,
			},
			Description: "Create nested directories",
		},
		{
			Input: map[string]any{
				"path": "existing_folder",
				"parents": true,
				"mode": "0755",
			},
			Description: "Create directory with specific permissions (no error if exists)",
		},
	}
}

func (t *MakeDirectoryTool) WhenToUse() string {
	return "Use to create new directories for organizing files. Use 'parents=true' to create all parent directories automatically."
}

func (t *MakeDirectoryTool) ChainsWith() []string {
	return []string{"write_file", "move", "copy"}
}

func (t *MakeDirectoryTool) ValidateArgs(args map[string]any) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("missing or invalid 'path' argument")
	}

	return nil
}

func (t *MakeDirectoryTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, _ := args["path"].(string)
	parents, _ := args["parents"].(bool)
	modeStr, _ := args["mode"].(string)

	// Parse mode or use default
	mode := os.FileMode(0755) // Default: rwxr-xr-x
	if modeStr != "" {
		var perm uint32
		if _, err := fmt.Sscanf(modeStr, "%o", &perm); err == nil {
			mode = os.FileMode(perm)
		}
	}

	// Check if already exists
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("path exists but is not a directory: %s", path),
			}, nil
		}
		// Directory exists - this is OK if parents flag is set
		if parents {
			return types.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Directory already exists: %s", path),
				Data: map[string]any{
					"path":    path,
					"existed": true,
				},
			}, nil
		}
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("directory already exists: %s (use parents=true to ignore)", path),
		}, nil
	}

	// Create directory
	var err error
	if parents {
		err = os.MkdirAll(path, mode)
	} else {
		// Check parent exists
		parentDir := path[:len(path)-1]
		if parentDir == "" {
			parentDir = "."
		}
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("parent directory does not exist: %s (use parents=true to create)", parentDir),
			}, nil
		}
		err = os.Mkdir(path, mode)
	}

	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	output := fmt.Sprintf("Created directory: %s", path)
	if parents {
		output += " (with parents)"
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"path":     path,
			"mode":     mode.String(),
			"existed":  false,
		},
	}, nil
}
