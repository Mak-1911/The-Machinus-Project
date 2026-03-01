package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinus/cloud-agent/internal/types"
)

// FileReadTool Reads file contents
type FileReadTool struct {
	maxFileSize int64 // max file size in bytes (10MB by default)
	workDir     string // working directory for relative paths
}

// NewFileReadTool creates a new files read tool
func NewFileReadTool(maxFileSize int64) *FileReadTool {
	if maxFileSize <= 0 {
		maxFileSize = 10 * 1024 * 1024 // default 10mb
	}
	// Get current working directory
	workDir, _ := os.Getwd()
	return &FileReadTool{
		maxFileSize: maxFileSize,
		workDir:     workDir,
	}
}

// resolvePath resolves a path (relative or absolute) to an absolute path
func (t *FileReadTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workDir, path)
}

func (t *FileReadTool) Name() string {
	return "read_file"
}

func (t *FileReadTool) Description() string {
	return "Read the contents of a file. ALWAYS read a file BEFORE editing it with edit_file. Supports offset/limit for large files and both relative/absolute paths."
}

// Examples returns example usages
func (t *FileReadTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{"file_path": "main.go"},
			Description: "Read the main.go file from current directory",
		},
		{
			Input: map[string]any{"file_path": "README.md", "offset": 0, "limit": 50},
			Description: "Read first 50 lines of README.md",
		},
		{
			Input: map[string]any{"file_path": "./internal/config/config.go"},
			Description: "Read config file using relative path",
		},
	}
}

// WhenToUse returns when this tool should be used
func (t *FileReadTool) WhenToUse() string {
	return "ALWAYS use this BEFORE edit_file to understand existing code. Use when you need to examine file contents, understand code structure, or verify file contents before making changes. For large files, use offset/limit to read in chunks."
}

// ChainsWith returns tools that typically follow this tool
func (t *FileReadTool) ChainsWith() []string {
	return []string{"edit_file", "grep"}
}

func (t *FileReadTool) ValidateArgs(args map[string]any) error {
	path, ok := args["file_path"].(string)
	if !ok || path == ""{
		return fmt.Errorf("missing or invalid 'file_path' argument")
	}
	return nil
}

func (t *FileReadTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, ok := args["file_path"].(string)
	if !ok || path == "" {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'file_path' argument")
	}

	// Resolve path (handle relative paths)
	path = t.resolvePath(path)

	// Check file info
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
			Error:   fmt.Sprintf("failed to read file: %w", err),
		}, nil
	}

	// Check file size
	if info.Size() > t.maxFileSize {
		return types.ToolResult{
			Success: false,
			Error: fmt.Sprintf("file too large (%d bytes, max %d bytes)", info.Size()),
		}, nil
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %w", err),
		}, nil
	}

	// Apply offset and limit if provided
	offset := 0
	limit := len(content)

	if off, ok := args["offset"].(int); ok && off > 0 {
		offset = off
	}

	if lim, ok := args["limit"].(int); ok && lim > 0 && lim < len(content)-offset {
		limit = offset + lim
	}

	// Ensure bounds
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	if limit > len(content) {
		limit = len(content)
	}
	if limit < offset {
		limit = offset
	}

	return types.ToolResult{
		Success: true,
		Output:  string(content[offset:limit]),
		Data: map[string]any{
			"file_path": path,
			"size":      info.Size(),
			"offset":    offset,
			"limit":     limit - offset,
		},
	}, nil
}

// FileWriteTool creates or overwrites files
type FileWriteTool struct {
	maxFileSize int64
	workDir     string
}

// NewFileWriteTool creates a new file write tool
func NewFileWriteTool(maxFileSize int64) *FileWriteTool {
	if maxFileSize <= 0 {
		maxFileSize = 10 * 1024 * 1024 // Default 10MB
	}
	workDir, _ := os.Getwd()
	return &FileWriteTool{
		maxFileSize: maxFileSize,
		workDir:     workDir,
	}
}

// resolvePath resolves a path (relative or absolute) to an absolute path
func (t *FileWriteTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workDir, path)
}

func (t *FileWriteTool) Name() string {
	return "write_file"
}

func (t *FileWriteTool) Description() string {
	return "Create a new file or completely overwrite an existing file. WARNING: This replaces ALL file contents. For partial edits, use edit_file instead. Supports both relative and absolute paths."
}

// Examples returns example usages
func (t *FileWriteTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{"file_path": "new_file.go", "content": "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}"},
			Description: "Create a new Go file",
		},
		{
			Input: map[string]any{"file_path": "README.md", "content": "# My Project\n\nThis is my project."},
			Description: "Create a README file",
		},
	}
}

// WhenToUse returns when this tool should be used
func (t *FileWriteTool) WhenToUse() string {
	return "Use ONLY when creating a new file or completely replacing a file's contents. For making small changes to existing files, use read_file FIRST, then edit_file. DO NOT use this for partial edits."
}

// ChainsWith returns tools that typically follow this tool
func (t *FileWriteTool) ChainsWith() []string {
	return []string{"read_file"}
}

func (t *FileWriteTool) ValidateArgs(args map[string]any) error {
	path, ok := args["file_path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("missing or invalid 'file_path' argument")
	}

	content, ok := args["content"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'content' argument")
	}

	if int64(len(content)) > t.maxFileSize {
		return fmt.Errorf("content too large (%d bytes, max %d bytes)", len(content), t.maxFileSize)
	}

	return nil
}

func (t *FileWriteTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, ok := args["file_path"].(string)
	if !ok || path == "" {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'file_path' argument")
	}

	content, ok := args["content"].(string)
	if !ok {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'content' argument")
	}

	// Resolve path (handle relative paths)
	path = t.resolvePath(path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to create directory: %w", err),
			}, nil
		}
	}

	// Write file
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write file: %w", err),
		}, nil
	}

	return types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path),
		Data: map[string]any{
			"file_path": path,
			"size":      len(content),
		},
	}, nil
}

// FileEditTool replaces text in files
type FileEditTool struct {
	maxFileSize int64
	workDir     string
}

// NewFileEditTool creates a new file edit tool
func NewFileEditTool(maxFileSize int64) *FileEditTool {
	if maxFileSize <= 0 {
		maxFileSize = 10 * 1024 * 1024 // Default 10MB
	}
	workDir, _ := os.Getwd()
	return &FileEditTool{
		maxFileSize: maxFileSize,
		workDir:     workDir,
	}
}

// resolvePath resolves a path (relative or absolute) to an absolute path
func (t *FileEditTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.workDir, path)
}

func (t *FileEditTool) Name() string {
	return "edit_file"
}

func (t *FileEditTool) Description() string {
	return "Replace specific text in a file. You MUST read the file first with read_file before using this tool. Replaces exact old_string match with new_string. Use replace_all=true for global replacement."
}

// Examples returns example usages
func (t *FileEditTool) Examples() []types.ToolExample{
	return []types.ToolExample{
		{
			Input: map[string]any{
				"file_path": "main.go",
				"old_string": "func oldName()",
				"new_string": "func newName()",
			},
			Description: "Rename a function",
		},
		{
			Input: map[string]any{
				"file_path": "config.json",
				"old_string": "\"port\": 8080",
				"new_string": "\"port\": 3000",
			},
			Description: "Change a configuration value",
		},
		{
			Input: map[string]any{
				"file_path": "app.go",
				"old_string": "fmt.Println",
				"new_string": "log.Println",
				"replace_all": true,
			},
			Description: "Replace all occurrences of fmt.Println with log.Println",
		},
	}
}

// WhenToUse returns when this tool should be used
func (t *FileEditTool) WhenToUse() string {
	return "Use AFTER reading a file with read_file to make targeted changes. Best for renaming variables/functions, changing specific values, or replacing exact text matches. ALWAYS read the file first to ensure old_string matches exactly."
}

// ChainsWith returns tools that typically follow this tool
func (t *FileEditTool) ChainsWith() []string {
	return []string{"read_file", "shell"}
}

func (t *FileEditTool) ValidateArgs(args map[string]any) error {
	path, ok := args["file_path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("missing or invalid 'file_path' argument")
	}

	oldStr, ok := args["old_string"].(string)
	if !ok || oldStr == "" {
		return fmt.Errorf("missing or invalid 'old_string' argument")
	}

	newStr, ok := args["new_string"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'new_string' argument")
	}

	if oldStr == newStr {
		return fmt.Errorf("old_string and new_string must be different")
	}

	return nil
}

func (t *FileEditTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, ok := args["file_path"].(string)
	if !ok || path == "" {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'file_path' argument")
	}

	oldStr, ok := args["old_string"].(string)
	if !ok || oldStr == "" {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'old_string' argument")
	}

	newStr, ok := args["new_string"].(string)
	if !ok {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'new_string' argument")
	}

	// Resolve path (handle relative paths)
	path = t.resolvePath(path)

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %w", err),
		}, nil
	}

	// Check file size
	if int64(len(content)) > t.maxFileSize {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("file too large (%d bytes, max %d bytes)", len(content), t.maxFileSize),
		}, nil
	}

	contentStr := string(content)

	// Perform replacement
	var newContent string
	replaceAll, ok := args["replace_all"].(bool)
	if ok && replaceAll {
		newContent = strings.ReplaceAll(contentStr, oldStr, newStr)
	} else {
		if !strings.Contains(contentStr, oldStr) {
			return types.ToolResult{
				Success: false,
				Error:   "old_string not found in file",
			}, nil
		}
		newContent = strings.Replace(contentStr, oldStr, newStr, 1)
	}

	// Write back
	err = os.WriteFile(path, []byte(newContent), 0644)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write file: %w", err),
		}, nil
	}

	return types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Successfully edited %s", path),
		Data: map[string]any{
			"file_path":    path,
			"replace_all":  replaceAll,
			"old_length":   len(oldStr),
			"new_length":   len(newStr),
			"size_change":  len(newContent) - len(contentStr),
		},
	}, nil
}