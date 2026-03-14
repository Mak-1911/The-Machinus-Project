// Package app provides tool execution for the LLM agent.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/tools"
	"github.com/machinus/cloud-agent/internal/types"
)

// ToolExecutor handles tool registration and execution for the LLM agent.
type ToolExecutor struct {
	tools  map[string]types.Tool
	workDir string
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor(workDir string) *ToolExecutor {
	te := &ToolExecutor{
		tools:   make(map[string]types.Tool),
		workDir: workDir,
	}
	te.registerDefaultTools()
	return te
}

// registerDefaultTools registers the default set of tools.
func (te *ToolExecutor) registerDefaultTools() {
	// Shell/Command execution
	te.RegisterTool("bash", tools.NewShellTool(te.workDir, 30*time.Second, true))
	te.RegisterTool("shell", tools.NewShellTool(te.workDir, 30*time.Second, true))

	// File operations
	te.RegisterTool("read_file", &ReadFileTool{workDir: te.workDir})
	te.RegisterTool("write_file", &WriteFileTool{workDir: te.workDir})
	te.RegisterTool("list_files", &ListFilesTool{workDir: te.workDir})
	te.RegisterTool("glob", &GlobTool{workDir: te.workDir})

	// Search
	te.RegisterTool("grep", &GrepTool{workDir: te.workDir})
}

// RegisterTool registers a tool with the executor.
func (te *ToolExecutor) RegisterTool(name string, tool types.Tool) {
	te.tools[name] = tool
}

// GetTools returns all registered tools.
func (te *ToolExecutor) GetTools() map[string]types.Tool {
	return te.tools
}

// GetTool returns a tool by name.
func (te *ToolExecutor) GetTool(name string) (types.Tool, bool) {
	tool, ok := te.tools[name]
	return tool, ok
}

// Execute executes a tool call and returns the result.
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any) (types.ToolResult, error) {
	tool, ok := te.tools[toolName]
	if !ok {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}

	// Validate arguments
	if err := tool.ValidateArgs(args); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	// Execute the tool
	return tool.Execute(ctx, args)
}

// GetToolsPrompt returns a prompt description of available tools for the LLM.
func (te *ToolExecutor) GetToolsPrompt() string {
	var sb strings.Builder
	sb.WriteString("You have access to the following tools. Use them when needed:\n\n")

	for name, tool := range te.tools {
		sb.WriteString(fmt.Sprintf("## %s\n", name))
		sb.WriteString(fmt.Sprintf("%s\n", tool.Description()))

		// Add examples if available
		if examples := tool.Examples(); len(examples) > 0 {
			sb.WriteString("\nExamples:\n")
			for _, ex := range examples {
				argsJSON, _ := json.Marshal(ex.Input)
				sb.WriteString(fmt.Sprintf("- %s: %s\n", ex.Description, string(argsJSON)))
			}
		}

		if when := tool.WhenToUse(); when != "" {
			sb.WriteString(fmt.Sprintf("\nWhen to use: %s\n", when))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nTool call format:\n")
	sb.WriteString("To call a tool, use one of these formats:\n\n")
	sb.WriteString("Format 1 - Multi-line:\n")
	sb.WriteString("```\n")
	sb.WriteString("tool:tool_name\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "arg1": "value1",` + "\n")
	sb.WriteString(`  "arg2": "value2"` + "\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Format 2 - Single-line:\n")
	sb.WriteString("```\n")
	sb.WriteString("tool:tool_name {\"arg1\": \"value1\"}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("IMPORTANT: Always use the 'tool:' prefix. JSON args are required.\n\n")

	sb.WriteString("For bash/shell commands, you can also use:\n\n")
	sb.WriteString("```\nbash\nyour command here\n```\n")
	sb.WriteString("or\n\n")
	sb.WriteString("```\nshell\nyour command here\n```\n\n")

	return sb.String()
}

// ============================================================================
// File Tools
// ============================================================================

// ReadFileTool reads a file's contents.
type ReadFileTool struct {
	workDir string
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Returns the full file content as text."
}

func (t *ReadFileTool) ValidateArgs(args map[string]any) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("missing 'path' argument")
	}
	return nil
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, _ := args["path"].(string)
	fullPath := filepath.Join(t.workDir, path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %v", err),
		}, nil
	}

	return types.ToolResult{
		Success: true,
		Output:  string(content),
		Data: map[string]any{
			"path":     path,
			"size":     len(content),
			"fullPath": fullPath,
		},
	}, nil
}

func (t *ReadFileTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{Input: map[string]any{"path": "README.md"}, Description: "Read README file"},
		{Input: map[string]any{"path": "src/main.go"}, Description: "Read Go source file"},
	}
}

func (t *ReadFileTool) WhenToUse() string {
	return "Use when you need to read the contents of a file. For listing files, use list_files instead."
}

func (t *ReadFileTool) ChainsWith() []string {
	return []string{"write_file", "edit_file", "grep"}
}

// WriteFileTool writes content to a file.
type WriteFileTool struct {
	workDir string
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, overwrites if it does."
}

func (t *WriteFileTool) ValidateArgs(args map[string]any) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return fmt.Errorf("missing 'path' argument")
	}
	content, ok := args["content"].(string)
	if !ok {
		return fmt.Errorf("missing 'content' argument")
	}
	_ = content
	return nil
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	fullPath := filepath.Join(t.workDir, path)

	// Create directory if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write file: %v", err),
		}, nil
	}

	return types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("File written: %s (%d bytes)", path, len(content)),
		Data: map[string]any{
			"path":     path,
			"size":     len(content),
			"fullPath": fullPath,
		},
	}, nil
}

func (t *WriteFileTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{Input: map[string]any{"path": "test.txt", "content": "Hello World"}, Description: "Create a text file"},
		{Input: map[string]any{"path": "config.json", "content": "{\"key\": \"value\"}"}, Description: "Create JSON config"},
	}
}

func (t *WriteFileTool) WhenToUse() string {
	return "Use when you need to create a new file or completely replace an existing file's contents."
}

func (t *WriteFileTool) ChainsWith() []string {
	return []string{"read_file", "bash"}
}

// ListFilesTool lists files in a directory.
type ListFilesTool struct {
	workDir string
}

func (t *ListFilesTool) Name() string {
	return "list_files"
}

func (t *ListFilesTool) Description() string {
	return "List files and directories in a given path. Returns names and basic info."
}

func (t *ListFilesTool) ValidateArgs(args map[string]any) error {
	// Path is optional - defaults to current directory
	return nil
}

func (t *ListFilesTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path := "." // default
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	fullPath := filepath.Join(t.workDir, path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to list directory: %v", err),
		}, nil
	}

	var files []string
	for _, entry := range entries {
		info := ""
		if entry.IsDir() {
			info = " (DIR)"
		}
		files = append(files, entry.Name()+info)
	}

	return types.ToolResult{
		Success: true,
		Output:  strings.Join(files, "\n"),
		Data: map[string]any{
			"path":   path,
			"count":  len(files),
			"files":  files,
		},
	}, nil
}

func (t *ListFilesTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{Input: map[string]any{}, Description: "List current directory"},
		{Input: map[string]any{"path": "src"}, Description: "List src directory"},
	}
}

func (t *ListFilesTool) WhenToUse() string {
	return "Use when you need to see what files are in a directory."
}

func (t *ListFilesTool) ChainsWith() []string {
	return []string{"read_file", "grep"}
}

// GlobTool finds files matching a pattern.
type GlobTool struct {
	workDir string
}

func (t *GlobTool) Name() string {
	return "glob"
}

func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern like '*.go' or '**/*.txt'."
}

func (t *GlobTool) ValidateArgs(args map[string]any) error {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return fmt.Errorf("missing 'pattern' argument")
	}
	return nil
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	path := t.workDir
	if p, ok := args["path"].(string); ok && p != "" {
		path = filepath.Join(t.workDir, p)
	}

	matches, err := filepath.Glob(filepath.Join(path, pattern))
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("glob error: %v", err),
		}, nil
	}

	// Make paths relative to workDir
	var results []string
	for _, match := range matches {
		relPath, err := filepath.Rel(t.workDir, match)
		if err != nil {
			relPath = match
		}
		results = append(results, relPath)
	}

	return types.ToolResult{
		Success: true,
		Output:  strings.Join(results, "\n"),
		Data: map[string]any{
			"pattern": pattern,
			"count":   len(results),
			"matches": results,
		},
	}, nil
}

func (t *GlobTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{Input: map[string]any{"pattern": "*.go"}, Description: "Find all Go files"},
		{Input: map[string]any{"pattern": "**/*.md"}, Description: "Find all markdown files recursively"},
	}
}

func (t *GlobTool) WhenToUse() string {
	return "Use when you need to find files by pattern."
}

func (t *GlobTool) ChainsWith() []string {
	return []string{"read_file", "grep"}
}

// GrepTool searches for text in files.
type GrepTool struct {
	workDir string
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search for text/pattern in files. Returns matching lines with context."
}

func (t *GrepTool) ValidateArgs(args map[string]any) error {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return fmt.Errorf("missing 'pattern' argument")
	}
	return nil
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	path := t.workDir
	if p, ok := args["path"].(string); ok && p != "" {
		path = filepath.Join(t.workDir, p)
	}

	var matches []string

	// Simple grep implementation
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if strings.Contains(line, pattern) {
				relPath, _ := filepath.Rel(t.workDir, filePath)
				matches = append(matches, fmt.Sprintf("%s:%d:%s", relPath, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})

	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("grep error: %v", err),
		}, nil
	}

	return types.ToolResult{
		Success: true,
		Output:  strings.Join(matches, "\n"),
		Data: map[string]any{
			"pattern": pattern,
			"count":   len(matches),
			"matches": matches,
		},
	}, nil
}

func (t *GrepTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{Input: map[string]any{"pattern": "TODO"}, Description: "Find TODO comments"},
		{Input: map[string]any{"pattern": "func main", "path": "src"}, Description: "Search in src directory"},
	}
}

func (t *GrepTool) WhenToUse() string {
	return "Use when you need to search for text across files."
}

func (t *GrepTool) ChainsWith() []string {
	return []string{"read_file", "glob"}
}
