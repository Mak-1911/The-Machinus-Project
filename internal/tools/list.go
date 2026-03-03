package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// ListTool lists directory contents
type ListTool struct{}

// walkItem represents a file/directory from a walk operation
type walkItem struct {
	info fs.FileInfo
	path string
}

// NewListTool creates a new list tool
func NewListTool() *ListTool {
	return &ListTool{}
}

func (t *ListTool) Name() string {
	return "list"
}

func (t *ListTool) Description() string {
	return "List directory contents with optional details (size, permissions, dates). Supports recursive listing and sorting."
}

func (t *ListTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"path": ".",
			},
			Description: "List current directory",
		},
		{
			Input: map[string]any{
				"path": "src/",
				"details": true,
			},
			Description: "List directory with file details (size, permissions, dates)",
		},
		{
			Input: map[string]any{
				"path": ".",
				"recursive": true,
				"details": true,
			},
			Description: "List all files recursively with details",
		},
	}
}

func (t *ListTool) WhenToUse() string {
	return "Use to explore directory structures, find files, check file sizes, or see directory contents. Use 'details=true' to see file metadata."
}

func (t *ListTool) ChainsWith() []string {
	return []string{"glob", "grep", "read_file", "copy"}
}

func (t *ListTool) ValidateArgs(args map[string]any) error {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		// Default to current directory
		return nil
	}

	return nil
}

func (t *ListTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	details, _ := args["details"].(bool)
	recursive, _ := args["recursive"].(bool)
	sortBy, _ := args["sort"].(string) // "name", "size", "date"

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot access path: %v", err),
		}, nil
	}

	// If it's a file, return its info
	if !info.IsDir() {
		fileInfo := t.formatFileInfo(path, info, details)
		return types.ToolResult{
			Success: true,
			Output:  fileInfo,
			Data: map[string]any{
				"items": []map[string]any{t.createFileInfoMap(path, info)},
				"count": 1,
			},
		}, nil
	}

	// List directory
	var items []walkItem
	var allPaths []string

	if recursive {
		err = filepath.Walk(path, func(subPath string, info fs.FileInfo, err error) error {
			if err != nil {
				return nil // Continue on error
			}
			if subPath != path { // Skip the root directory
				items = append(items, walkItem{info: info, path: subPath})
				allPaths = append(allPaths, subPath)
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(path)
		if err != nil {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to list directory: %v", err),
			}, nil
		}
		for _, entry := range entries {
			itemPath := filepath.Join(path, entry.Name())
			if info, err := entry.Info(); err == nil {
				items = append(items, walkItem{info: info, path: itemPath})
				allPaths = append(allPaths, itemPath)
			}
		}
	}

	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to list directory: %v", err),
		}, nil
	}

	// Sort items if requested
	if sortBy != "" {
		t.sortWalkItems(items, allPaths, sortBy)
	}

	// Format output
	output := fmt.Sprintf("Listing: %s\n", path)
	output += fmt.Sprintf("Items: %d\n\n", len(items))

	if details {
		output += t.formatDetailedWalkList(items)
	} else {
		output += t.formatSimpleWalkList(items)
	}

	// Create data array
	dataArray := make([]map[string]any, 0)
	for _, item := range items {
		dataArray = append(dataArray, t.createFileInfoMap(item.path, item.info))
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"path":  path,
			"items": dataArray,
			"count": len(items),
		},
	}, nil
}

func (t *ListTool) formatSimpleList(items []fs.DirEntry) string {
	result := ""
	for _, item := range items {
		name := item.Name()
		if item.IsDir() {
			name += "/"
		}
		result += fmt.Sprintf("  %s\n", name)
	}
	return result
}

func (t *ListTool) formatSimpleWalkList(items []walkItem) string {
	result := ""
	for _, item := range items {
		name := filepath.Base(item.path)
		if item.info.IsDir() {
			name += "/"
		}
		result += fmt.Sprintf("  %s\n", name)
	}
	return result
}

func (t *ListTool) formatDetailedList(items []fs.DirEntry, paths []string) string {
	result := fmt.Sprintf("%-10s %-12s %-15s %-20s %s\n",
		"Type", "Size", "Permissions", "Modified", "Name")
	result += fmt.Sprintf("%s\n", "─"+strings.Repeat("─", 75))

	for i, item := range items {
		info, err := item.Info()
		if err != nil {
			continue
		}

		result += t.formatFileInfo(paths[i], info, true) + "\n"
	}

	return result
}

func (t *ListTool) formatDetailedWalkList(items []walkItem) string {
	result := fmt.Sprintf("%-10s %-12s %-15s %-20s %s\n",
		"Type", "Size", "Permissions", "Modified", "Name")
	result += fmt.Sprintf("%s\n", "─"+strings.Repeat("─", 75))

	for _, item := range items {
		result += t.formatFileInfo(item.path, item.info, true) + "\n"
	}

	return result
}

func (t *ListTool) formatFileInfo(path string, info os.FileInfo, details bool) string {
	if !details {
		name := filepath.Base(path)
		if info.IsDir() {
			name += "/"
		}
		return name
	}

	// File type
	fileType := "file"
	if info.IsDir() {
		fileType = "dir"
	}

	// Size
	size := info.Size()
	sizeStr := t.formatSize(size)

	// Permissions
	permStr := info.Mode().String()

	// Modified time
	modTime := info.ModTime().Format("2006-01-02 15:04:05")

	// Name
	name := filepath.Base(path)
	if info.IsDir() {
		name += "/"
	}

	return fmt.Sprintf("%-10s %-12s %-15s %-20s %s",
		fileType, sizeStr, permStr, modTime, name)
}

func (t *ListTool) createFileInfoMap(path string, info os.FileInfo) map[string]any {
	return map[string]any{
		"name":        filepath.Base(path),
		"path":        path,
		"type":        t.getFileType(info),
		"size":        info.Size(),
		"permissions": info.Mode().String(),
		"modified":    info.ModTime().Format(time.RFC3339),
		"is_dir":      info.IsDir(),
	}
}

func (t *ListTool) getFileType(info os.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	mode := info.Mode()
	if mode&fs.ModeSymlink != 0 {
		return "symlink"
	}
	return "file"
}

func (t *ListTool) formatSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1fG", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.1fM", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1fK", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%dB", size)
	}
}

func (t *ListTool) sortItems(items []fs.DirEntry, paths []string, sortBy string) {
	// Create sortable indices
	type itemSort struct {
		index int
		name  string
		size  int64
		date  time.Time
	}

	sortables := make([]itemSort, len(items))
	for i, item := range items {
		info, _ := item.Info()
		sortables[i] = itemSort{
			index: i,
			name:  item.Name(),
			size:  info.Size(),
			date:  info.ModTime(),
		}
	}

	sort.Slice(sortables, func(i, j int) bool {
		switch sortBy {
		case "size":
			return sortables[i].size > sortables[j].size // Largest first
		case "date":
			return sortables[i].date.After(sortables[j].date) // Newest first
		default: // name
			return sortables[i].name < sortables[j].name // Alphabetical
		}
	})

	// Reorder items and paths based on sort
	sortedItems := make([]fs.DirEntry, len(items))
	sortedPaths := make([]string, len(paths))
	for i, s := range sortables {
		sortedItems[i] = items[s.index]
		sortedPaths[i] = paths[s.index]
	}

	copy(items, sortedItems)
	copy(paths, sortedPaths)
}

func (t *ListTool) sortWalkItems(items []walkItem, paths []string, sortBy string) {
	// Create sortable indices
	type itemSort struct {
		index int
		name  string
		size  int64
		date  time.Time
	}

	sortables := make([]itemSort, len(items))
	for i, item := range items {
		sortables[i] = itemSort{
			index: i,
			name:  filepath.Base(item.path),
			size:  item.info.Size(),
			date:  item.info.ModTime(),
		}
	}

	sort.Slice(sortables, func(i, j int) bool {
		switch sortBy {
		case "size":
			return sortables[i].size > sortables[j].size // Largest first
		case "date":
			return sortables[i].date.After(sortables[j].date) // Newest first
		default: // name
			return sortables[i].name < sortables[j].name // Alphabetical
		}
	})

	// Reorder items and paths based on sort
	sortedItems := make([]walkItem, len(items))
	sortedPaths := make([]string, len(paths))
	for i, s := range sortables {
		sortedItems[i] = items[s.index]
		sortedPaths[i] = paths[s.index]
	}

	copy(items, sortedItems)
	copy(paths, sortedPaths)
}
