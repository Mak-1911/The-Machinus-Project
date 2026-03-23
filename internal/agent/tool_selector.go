package agent

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/machinus/cloud-agent/internal/types"
)

// TaskPattern represents a recognizable pattern in user requests
type TaskPattern struct {
	Name        string
	Patterns    []string // Regex patterns to match
	Confidence  float64  // Base confidence for this pattern
	ToolChain   []string // Suggested tool chain
	Constraints []string // Additional context constraints
}

// ToolSuggestion represents a suggested tool or tool chain
type ToolSuggestion struct {
	Tool       string
	Args       map[string]any
	Confidence float64
	Reason     string
}

// ToolSelector provides heuristic-based tool selection
type ToolSelector struct {
	tools      map[string]types.Tool
	patterns   []TaskPattern
	metrics    map[string]*ToolMetrics
	mu         sync.RWMutex

	// Confidence threshold for suggesting tools
	suggestionThreshold float64
}

// ToolMetrics tracks performance metrics for tools
type ToolMetrics struct {
	TotalCalls      int64
	SuccessCount    int64
	FailureCount    int64
	TotalDurationMs int64
	LastUsed        int64 // Unix timestamp

	// Error breakdown
	ErrorTypes map[string]int64

	// Average success rate
	SuccessRate float64
}

// NewToolSelector creates a new heuristic tool selector
func NewToolSelector(tools map[string]types.Tool) *ToolSelector {
	ts := &ToolSelector{
		tools:               tools,
		metrics:             make(map[string]*ToolMetrics),
		suggestionThreshold: 0.6, // Suggest only if confidence >= 60%
	}

	// Initialize metrics for all tools
	for name := range tools {
		ts.metrics[name] = &ToolMetrics{
			ErrorTypes: make(map[string]int64),
		}
	}

	// Initialize patterns
	ts.initPatterns()

	return ts
}

// initPatterns initializes the task patterns for heuristic matching
func (ts *ToolSelector) initPatterns() {
	ts.patterns = []TaskPattern{
		// File discovery patterns
		{
			Name: "find_files_by_extension",
			Patterns: []string{
				`(?i)find.*\.(go|js|ts|py|java|cpp|c|h|rs|md|txt|json|yaml|yml|xml)`,
				`(?i)list.*all.*\.(go|js|ts|py)`,
				`(?i)show.*\.(go|js|ts|py).*files`,
				`(?i)\*+\.(go|js|ts|py|java|cpp|c|h|rs)`,
			},
			Confidence: 0.9,
			ToolChain:  []string{"glob"},
		},
		{
			Name: "find_files_recursive",
			Patterns: []string{
				`(?i)find.*all.*files`,
				`(?i)list.*recursive`,
				`(?i)show.*all.*files.*recursive`,
				`(?i)search.*files.*tree`,
			},
			Confidence: 0.85,
			ToolChain:  []string{"glob"},
		},

		// Search patterns
		{
			Name: "search_text_in_files",
			Patterns: []string{
				`(?i)search.*for.*["'](.+)["'].*in`,
				`(?i)find.*["'](.+)["'].*in.*files`,
				`(?i)grep.*for`,
				`(?i)where.*is.*["'](.+)["']`,
				`(?i)look.*for.*["'](.+)["']`,
			},
			Confidence: 0.95,
			ToolChain:  []string{"grep"},
		},
		{
			Name: "search_code_pattern",
			Patterns: []string{
				`(?i)find.*function.*called`,
				`(?i)where.*defined`,
				`(?i)find.*usage.*of`,
				`(?i)search.*symbol`,
				`(?i)find.*reference`,
			},
			Confidence: 0.85,
			ToolChain:  []string{"grep"},
		},

		// File read patterns
		{
			Name: "read_specific_file",
			Patterns: []string{
				`(?i)read.*file.*["']?([^"'\s]+)["']?`,
				`(?i)show.*me.*["']?([^"'\s]+)["']?`,
				`(?i)what.*in.*["']?([^"'\s]+)["']?`,
				`(?i)open.*file`,
				`(?i)cat.*["']?([^"'\s]+)["']?`,
			},
			Confidence: 0.9,
			ToolChain:  []string{"read_file"},
		},

		// File edit patterns
		{
			Name: "edit_file",
			Patterns: []string{
				`(?i)change.*["'](.+)["'].*to`,
				`(?i)replace.*["'](.+)["'].*with`,
				`(?i)modify.*file`,
				`(?i)update.*["']?([^"'\s]+)["']?`,
				`(?i)fix.*in.*file`,
			},
			Confidence: 0.85,
			ToolChain:  []string{"read_file", "edit_file"},
		},

		// Web requests
		{
			Name: "http_request",
			Patterns: []string{
				`(?i)fetch.*https?://`,
				`(?i)call.*api`,
				`(?i)make.*request.*to`,
				`(?i)get.*from.*https?://`,
				`(?i)post.*to`,
			},
			Confidence: 0.95,
			ToolChain:  []string{"http"},
		},

		// Browser automation
		{
			Name: "browser_automation",
			Patterns: []string{
				`(?i)open.*browser`,
				`(?i)navigate.*to`,
				`(?i)click.*on.*page`,
				`(?i)screenshot.*site`,
				`(?i)scrape.*website.*dynamic`,
			},
			Confidence: 0.9,
			ToolChain:  []string{"browser"},
		},

		// Shell commands
		{
			Name: "shell_command",
			Patterns: []string{
				`(?i)run.*command`,
				`(?i)execute.*shell`,
				`(?i)` + "`" + `.*` + "`",
				`(?i)bash`,
				`(?i)sh -c`,
			},
			Confidence: 0.85,
			ToolChain:  []string{"shell"},
		},

		// Directory operations
		{
			Name: "list_directory",
			Patterns: []string{
				`(?i)what.*files.*in`,
				`(?i)list.*directory`,
				`(?i)ls.*`,
				`(?i)dir.*`,
				`(?i)show.*contents`,
			},
			Confidence: 0.9,
			ToolChain:  []string{"list"},
		},

		// Multi-step patterns
		{
			Name: "find_then_read",
			Patterns: []string{
				`(?i)find.*and.*read`,
				`(?i)search.*then.*show`,
				`(?i)locate.*and.*display`,
			},
			Confidence: 0.8,
			ToolChain:  []string{"glob", "read_file"},
		},

		{
			Name: "find_then_search",
			Patterns: []string{
				`(?i)find.*files.*then.*search`,
				`(?i)in.*\.(go|js|ts).*search.*for`,
			},
			Confidence: 0.85,
			ToolChain:  []string{"glob", "grep"},
		},

		// Code refactoring patterns
		{
			Name: "refactor_rename",
			Patterns: []string{
				`(?i)rename.*function`,
				`(?i)rename.*variable.*across`,
				`(?i)change.*name.*in.*all.*files`,
			},
			Confidence: 0.8,
			ToolChain:  []string{"glob", "grep", "read_file", "edit_file"},
		},

		// Write/create patterns
		{
			Name: "write_file",
			Patterns: []string{
				`(?i)create.*file`,
				`(?i)write.*to`,
				`(?i)save.*as`,
				`(?i)new.*file`,
				`(?i)generate.*file`,
			},
			Confidence: 0.85,
			ToolChain:  []string{"write_file"},
		},

		// Move/copy patterns
		{
			Name: "move_copy",
			Patterns: []string{
				`(?i)move.*file`,
				`(?i)copy.*file`,
				`(?i)rename.*file`,
				`(?i)transfer.*to`,
			},
			Confidence: 0.85,
			ToolChain:  []string{"copy"}, // copy handles both copy and move
		},

		// Delete patterns
		{
			Name: "delete_file",
			Patterns: []string{
				`(?i)delete.*file`,
				`(?i)remove.*file`,
				`(?i)rm.*`,
			},
			Confidence: 0.9,
			ToolChain:  []string{"delete"},
		},
	}
}

// AnalyzeRequest analyzes a user request and returns tool suggestions
func (ts *ToolSelector) AnalyzeRequest(request string) []ToolSuggestion {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var suggestions []ToolSuggestion
	request = strings.TrimSpace(request)

	// Check each pattern
	for _, pattern := range ts.patterns {
		if match, groups := ts.matchPattern(pattern, request); match {
			confidence := ts.calculateConfidence(pattern, groups)

			if confidence >= ts.suggestionThreshold {
				// Build suggestions for the tool chain
				for i, toolName := range pattern.ToolChain {
					// Skip if tool doesn't exist
					if _, ok := ts.tools[toolName]; !ok {
						continue
					}

					// Higher confidence for first tool in chain
					toolConfidence := confidence
					if i > 0 {
						toolConfidence = confidence * 0.9
					}

					// Extract suggested arguments
					args := ts.extractArgs(request, toolName, groups)

					suggestions = append(suggestions, ToolSuggestion{
						Tool:       toolName,
						Args:       args,
						Confidence: toolConfidence,
						Reason:     fmt.Sprintf("Pattern match: %s", pattern.Name),
					})
				}
			}
		}
	}

	// Adjust confidence based on tool metrics
	for i := range suggestions {
		suggestions[i].Confidence = ts.adjustForMetrics(suggestions[i])
	}

	return suggestions
}

// matchPattern checks if a pattern matches the request
func (ts *ToolSelector) matchPattern(pattern TaskPattern, request string) (bool, map[string]string) {
	for _, pat := range pattern.Patterns {
		re, err := regexp.Compile(pat)
		if err != nil {
			continue
		}

		if re.MatchString(request) {
			// Extract named groups if any
			matches := re.FindStringSubmatch(request)
			groups := make(map[string]string)

			// Get subexp names
			names := re.SubexpNames()
			for i, name := range names {
				if i > 0 && i < len(matches) && name != "" {
					groups[name] = matches[i]
				}
			}

			return true, groups
		}
	}

	// Fallback: check if keywords are present
	return ts.fuzzyMatch(pattern, request), nil
}

// fuzzyMatch performs a fuzzy keyword match
func (ts *ToolSelector) fuzzyMatch(pattern TaskPattern, request string) bool {
	requestLower := strings.ToLower(request)

	for _, pat := range pattern.Patterns {
		// Extract keywords from pattern (words without regex syntax)
		keywords := strings.Fields(strings.ToLower(regexp.MustCompile(`[^\w\s]`).ReplaceAllString(pat, " ")))

		matchedCount := 0
		for _, keyword := range keywords {
			if len(keyword) > 2 && strings.Contains(requestLower, keyword) {
				matchedCount++
			}
		}

		// Require at least 2 keywords to match, or 80% of keywords
		if matchedCount >= 2 || (len(keywords) > 0 && float64(matchedCount)/float64(len(keywords)) >= 0.8) {
			return true
		}
	}

	return false
}

// calculateConfidence calculates the confidence score for a matched pattern
func (ts *ToolSelector) calculateConfidence(pattern TaskPattern, groups map[string]string) float64 {
	confidence := pattern.Confidence

	// Boost confidence if we captured specific groups
	if len(groups) > 0 {
		confidence = min(confidence+0.1, 1.0)
	}

	return confidence
}

// extractArgs extracts suggested arguments from the request
func (ts *ToolSelector) extractArgs(request, toolName string, groups map[string]string) map[string]any {
	args := make(map[string]any)

	switch toolName {
	case "glob":
		// Extract file pattern
		if ext := ts.extractExtension(request); ext != "" {
			args["pattern"] = "*." + ext
		} else if pattern := ts.extractQuoted(request); pattern != "" {
			args["pattern"] = pattern
		} else {
			args["pattern"] = "*"
		}

	case "grep":
		// Extract search term
		if term := ts.extractQuoted(request); term != "" {
			args["pattern"] = term
		} else if search := ts.extractAfter(request, "for", "in", "search"); search != "" {
			args["pattern"] = search
		}

		// Extract file pattern if specified
		if ext := ts.extractExtension(request); ext != "" {
			args["glob"] = "*." + ext
		}

	case "read_file", "edit_file", "fileinfo":
		// Extract file path
		if path := ts.extractQuoted(request); path != "" {
			args["file_path"] = path
		} else if path := ts.extractFilePath(request); path != "" {
			args["file_path"] = path
		}

	case "http":
		// Extract URL
		if url := ts.extractURL(request); url != "" {
			args["url"] = url
			args["method"] = "GET"
		}

	case "shell":
		// Extract command
		if cmd := ts.extractCommand(request); cmd != "" {
			args["cmd"] = cmd
		}

	case "list":
		// Extract path
		if path := ts.extractFilePath(request); path != "" {
			args["path"] = path
		}
	}

	return args
}

// adjustForMetrics adjusts confidence based on historical tool performance
func (ts *ToolSelector) adjustForMetrics(suggestion ToolSuggestion) float64 {
	metric, ok := ts.metrics[suggestion.Tool]
	if !ok || metric.TotalCalls < 5 {
		// Not enough data, return original confidence
		return suggestion.Confidence
	}

	// Adjust based on success rate
	if metric.SuccessRate < 0.5 {
		// Low success rate, reduce confidence
		return suggestion.Confidence * 0.7
	} else if metric.SuccessRate > 0.9 {
		// High success rate, boost confidence
		return min(suggestion.Confidence*1.1, 1.0)
	}

	return suggestion.Confidence
}

// SuggestAlternatives suggests alternative tools when a tool fails
func (ts *ToolSelector) SuggestAlternatives(failedTool string, error string, originalArgs map[string]any) []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	alternatives := []string{}

	// Check if tool exists
	_, ok := ts.tools[failedTool]
	if !ok {
		return alternatives
	}

	// Add heuristic alternatives based on error type
	errorLower := strings.ToLower(error)

	switch failedTool {
	case "http":
		if strings.Contains(errorLower, "timeout") || strings.Contains(errorLower, "network") {
			alternatives = append(alternatives, "browser")
		}

	case "browser":
		if strings.Contains(errorLower, "not running") || strings.Contains(errorLower, "unavailable") {
			alternatives = append(alternatives, "http")
		}

	case "grep":
		if strings.Contains(errorLower, "too many") || strings.Contains(errorLower, "binary") {
			// Try with head_limit
			if originalArgs["head_limit"] == nil {
				// Suggest retry with limit
				alternatives = append(alternatives, "grep:limited")
			}
			// Or try shell-based grep
			alternatives = append(alternatives, "shell")
		}

	case "glob":
		if strings.Contains(errorLower, "too many") {
			alternatives = append(alternatives, "list")
		}

	case "read_file":
		if strings.Contains(errorLower, "too large") {
			alternatives = append(alternatives, "shell") // for head/tail
		} else if strings.Contains(errorLower, "not found") {
			alternatives = append(alternatives, "glob") // to find similar files
		}
	}

	return ts.deduplicateAlternatives(alternatives)
}

// deduplicateAlternatives removes duplicates from alternatives list
func (ts *ToolSelector) deduplicateAlternatives(alts []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, alt := range alts {
		if !seen[alt] {
			seen[alt] = true
			result = append(result, alt)
		}
	}

	return result
}

// RecordResult records the result of a tool execution for metrics
func (ts *ToolSelector) RecordResult(tool string, success bool, durationMs int64, errorType types.FailureType) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	metric, ok := ts.metrics[tool]
	if !ok {
		metric = &ToolMetrics{
			ErrorTypes: make(map[string]int64),
		}
		ts.metrics[tool] = metric
	}

	metric.TotalCalls++
	metric.TotalDurationMs += durationMs
	metric.LastUsed = 0 // TODO: set actual timestamp

	if success {
		metric.SuccessCount++
	} else {
		metric.FailureCount++
		if errorType != "" {
			metric.ErrorTypes[string(errorType)]++
		}
	}

	// Update success rate
	if metric.TotalCalls > 0 {
		metric.SuccessRate = float64(metric.SuccessCount) / float64(metric.TotalCalls)
	}
}

// GetMetrics returns the metrics for a tool
func (ts *ToolSelector) GetMetrics(tool string) *ToolMetrics {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.metrics[tool]
}

// ShouldOverrideLLM checks if heuristic confidence is high enough to override LLM
func (ts *ToolSelector) ShouldOverrideLLM(suggestions []ToolSuggestion, llmChoice string) (bool, string) {
	if len(suggestions) == 0 {
		return false, ""
	}

	topSuggestion := suggestions[0]

	// If our confidence is very high (>0.9), override
	if topSuggestion.Confidence > 0.9 {
		return true, fmt.Sprintf("High confidence (%.0f%%) pattern match suggests: %s",
			topSuggestion.Confidence*100, topSuggestion.Tool)
	}

	// If LLM choice is low-confidence (not in our suggestions), suggest our top pick
	llmInSuggestions := false
	for _, s := range suggestions {
		if s.Tool == llmChoice {
			llmInSuggestions = true
			break
		}
	}

	if !llmInSuggestions && topSuggestion.Confidence > 0.75 {
		return true, fmt.Sprintf("LLM choice '%s' not in heuristic suggestions (%.0f%% confidence: %s)",
			llmChoice, topSuggestion.Confidence*100, topSuggestion.Tool)
	}

	return false, ""
}

// Helper methods for extraction

func (ts *ToolSelector) extractExtension(s string) string {
	re := regexp.MustCompile(`\.(\w+)`)
	matches := re.FindAllStringSubmatch(s, -1)
	// Return the most common/last extension
	for _, m := range matches {
		if len(m) > 1 {
			ext := m[1]
			// Common extensions
			commonExts := map[string]bool{
				"go": true, "js": true, "ts": true, "py": true, "java": true,
				"cpp": true, "c": true, "h": true, "rs": true, "rb": true,
				"md": true, "txt": true, "json": true, "yaml": true, "yml": true,
				"xml": true, "html": true, "css": true, "sh": true, "sql": true,
			}
			if commonExts[ext] {
				return ext
			}
		}
	}
	return ""
}

func (ts *ToolSelector) extractQuoted(s string) string {
	re := regexp.MustCompile(`["']([^"']+)["']`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (ts *ToolSelector) extractURL(s string) string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	return re.FindString(s)
}

func (ts *ToolSelector) extractFilePath(s string) string {
	// Try to extract file paths like "./file.go", "path/to/file", "/absolute/path"
	re := regexp.MustCompile(`[~./]?[\w/-]+\.[\w]+|[~./]?[\w-]+/[\w/-]+`)
	matches := re.FindAllString(s, -1)
	if len(matches) > 0 {
		// Return the longest match (most likely to be the full path)
		longest := matches[0]
		for _, m := range matches {
			if len(m) > len(longest) {
				longest = m
			}
		}
		return longest
	}
	return ""
}

func (ts *ToolSelector) extractCommand(s string) string {
	// Extract commands in backticks
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}

	// Or extract after "run", "execute", "command"
	for _, prefix := range []string{"run ", "execute ", "command ", "cmd "} {
		if idx := strings.Index(strings.ToLower(s), prefix); idx != -1 {
			cmd := strings.TrimSpace(s[idx+len(prefix):])
			if cmd != "" {
				return cmd
			}
		}
	}

	return ""
}

func (ts *ToolSelector) extractAfter(s string, keywords ...string) string {
	sLower := strings.ToLower(s)
	for _, keyword := range keywords {
		if idx := strings.Index(sLower, keyword); idx != -1 {
			after := strings.TrimSpace(s[idx+len(keyword):])
			// Extract until next delimiter
			if idx := strings.IndexAny(after, ".,;"); idx != -1 {
				return strings.TrimSpace(after[:idx])
			}
			return after
		}
	}
	return ""
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
