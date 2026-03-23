package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/machinus/cloud-agent/internal/types"
)

// GlobTool finds files by pattern
type GlobTool struct {
	workDir    string
	maxResults int
}

// NewGlobTool creates a new glob tool
func NewGlobTool(maxResults int) *GlobTool {
	if maxResults <= 0 {
		maxResults = 1000 // Default max results
	}
	workDir, _ := os.Getwd()
	return &GlobTool{
		workDir:    workDir,
		maxResults: maxResults,
	}
}

func (t *GlobTool) Name() string {
	return "glob"
}

func (t *GlobTool) Description() string {
	return "Find files by name pattern using glob matching. Use to discover files, explore project structure, or find files matching a pattern. Supports *.txt, **/*.go, src/**/*.js patterns."
}

// Examples returns example usages
func (t *GlobTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{"pattern": "*.go"},
			Description: "Find all Go files in current directory",
		},
		{
			Input: map[string]any{"pattern": "**/*.go"},
			Description: "Find all Go files recursively",
		},
		{
			Input: map[string]any{"pattern": "*.txt", "path": "./docs"},
			Description: "Find all txt files in docs directory",
		},
		{
			Input: map[string]any{"pattern": "README*"},
			Description: "Find files starting with README",
		},
	}
}

// WhenToUse returns when this tool should be used
func (t *GlobTool) WhenToUse() string {
	return "Use to discover files in a project, find files by name pattern, or explore directory structure. ALWAYS start with glob when you need to find files before reading them. Use grep to search file contents instead of names."
}

// ChainsWith returns tools that typically follow this tool
func (t *GlobTool) ChainsWith() []string {
	return []string{"read_file", "grep"}
}

func (t *GlobTool) ValidateArgs(args map[string]any) error {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return fmt.Errorf("missing or invalid 'pattern' argument")
	}
	return nil
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'pattern' argument")
	}

	// Get search path
	searchPath := t.workDir
	if path, ok := args["path"].(string); ok && path != "" {
		if filepath.IsAbs(path) {
			searchPath = path
		} else {
			searchPath = filepath.Join(t.workDir, path)
		}
	}

	var matches []string
	count := 0

	// Walk the directory tree
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if count >= t.maxResults {
			return fmt.Errorf("max results reached")
		}

		// Get relative path for matching
		relPath, err := filepath.Rel(searchPath, path)
		if err != nil {
			return nil
		}

		// Convert to forward slashes for pattern matching (glob standard)
		relPath = filepath.ToSlash(relPath)

		// Match against pattern
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return nil
		}

		// Also check if pattern contains path separators (like **/*.go)
		if !matched && strings.Contains(pattern, "/") {
			matched, _ = filepath.Match(pattern, relPath)
		}

		// Handle ** pattern (recursive)
		if !matched && strings.Contains(pattern, "**") {
			patternRegex := "^" + strings.ReplaceAll(pattern, "**", ".*") + "$"
			matched, _ = regexp.MatchString(patternRegex, relPath)
		}

		if matched {
			matches = append(matches, path)
			count++
		}

		return nil
	})

	// If we hit max results, that's OK - just truncate
	if err != nil && err.Error() != "max results reached" {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to walk directory: %w", err),
		}, nil
	}

	// Format output
	output := fmt.Sprintf("Found %d file(s) matching '%s':\n", len(matches), pattern)
	for _, match := range matches {
		relPath, _ := filepath.Rel(searchPath, match)
		output += fmt.Sprintf("  - %s\n", relPath)
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"pattern": pattern,
			"path":    searchPath,
			"matches": matches,
			"count":   len(matches),
		},
	}, nil
}

// GrepTool searches file contents
type GrepTool struct {
	workDir    string
	maxResults int
}

// NewGrepTool creates a new grep tool
func NewGrepTool(maxResults int) *GrepTool {
	if maxResults <= 0 {
		maxResults = 1000
	}
	workDir, _ := os.Getwd()
	return &GrepTool{
		workDir:    workDir,
		maxResults: maxResults,
	}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search for text patterns inside file contents using regex. Use to find specific code, functions, or text across multiple files. Supports case-insensitive search and file filtering."
}

// Examples returns example usages
func (t *GrepTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{"pattern": "func main", "glob": "*.go"},
			Description: "Find 'func main' in all Go files",
		},
		{
			Input: map[string]any{"pattern": "TODO:", "glob": "*.go", "-i": true},
			Description: "Find TODO comments case-insensitively",
		},
		{
			Input: map[string]any{"pattern": "import.*test", "glob": "*.go"},
			Description: "Find imports containing 'test' using regex",
		},
		{
			Input: map[string]any{"pattern": "error", "path": "./internal", "head_limit": 20},
			Description: "Find 'error' in internal directory, max 20 results",
		},
	}
}

// WhenToUse returns when this tool should be used
func (t *GrepTool) WhenToUse() string {
	return "Use to search for specific text, functions, or patterns inside file contents. Best for finding where functions are defined, finding usage of a variable, or searching for specific text. Use glob to find files by name instead."
}

// ChainsWith returns tools that typically follow this tool
func (t *GrepTool) ChainsWith() []string {
	return []string{"read_file", "edit_file"}
}

func (t *GrepTool) ValidateArgs(args map[string]any) error {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return fmt.Errorf("missing or invalid 'pattern' argument")
	}
	return nil
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return types.ToolResult{}, fmt.Errorf("missing or invalid 'pattern' argument")
	}

	// Compile regex
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid regex pattern: %w", err),
		}, nil
	}

	// Get search path
	searchPath := t.workDir
	if path, ok := args["path"].(string); ok && path != "" {
		if filepath.IsAbs(path) {
			searchPath = path
		} else {
			searchPath = filepath.Join(t.workDir, path)
		}
	}

	// Get glob filter
	globPattern := "*"
	if glob, ok := args["glob"].(string); ok && glob != "" {
		globPattern = glob
	}

	// Get options
	caseInsensitive := false
	if ci, ok := args["-i"].(bool); ok {
		caseInsensitive = ci
	}

	// Get head limit
	headLimit := 0
	if limit, ok := args["head_limit"].(int); ok && limit > 0 {
		headLimit = limit
	}

	// Binary file extensions to skip
	binaryExts := map[string]bool{
		".db": true, ".sqlite": true, ".sqlite3": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true, ".7z": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wav": true,
		".bin": true, ".dat": true,
	}

	// Directories to skip entirely
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		".crush": true, "target": true, "build": true, "dist": true,
		".cache": true, "__pycache__": true, "bin": true, ".vscode": true,
		".idea": true, ".claude": true,
	}

	type Match struct {
		File    string
		Line    int
		Content string
	}

	var matches []Match
	matchCount := 0

	// Walk directory
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip certain directories entirely
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check head limit
		if headLimit > 0 && matchCount >= headLimit {
			return fmt.Errorf("max results reached")
		}

		// Skip binary files by extension
		ext := strings.ToLower(filepath.Ext(path))
		if binaryExts[ext] {
			return nil
		}

		// Apply glob filter to filename
		matched, _ := filepath.Match(globPattern, filepath.Base(path))
		if !matched {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		// Quick binary check - skip files with null bytes
		for _, b := range content {
			if b == 0 {
				return nil // Skip binary file
			}
		}

		// Search line by line
		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			searchContent := line
			if caseInsensitive {
				searchContent = strings.ToLower(line)
			}

			if regex.MatchString(searchContent) {
				relPath, _ := filepath.Rel(searchPath, path)
				matches = append(matches, Match{
					File:    relPath,
					Line:    lineNum + 1,
					Content: strings.TrimSpace(line),
				})
				matchCount++
			}
		}

		return nil
	})

	// Handle max results
	if err != nil && err.Error() != "max results reached" {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to search: %w", err),
		}, nil
	}

	// Format output
	output := fmt.Sprintf("Found %d match(es) for pattern '%s':\n", len(matches), pattern)
	for _, match := range matches {
		output += fmt.Sprintf("%s:%d: %s\n", match.File, match.Line, match.Content)
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"pattern": pattern,
			"path":    searchPath,
			"matches": matches,
			"count":   len(matches),
		},
	}, nil
}
