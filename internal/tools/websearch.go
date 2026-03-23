package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// WebSearchTool performs web searches using DuckDuckGo API
type WebSearchTool struct {
	timeout     time.Duration
	maxResults  int
	userAgent   string
	apiEndpoint string
}

// DuckDuckGoInstantAnswer represents the response from DuckDuckGo API
type DuckDuckGoInstantAnswer struct {
	Abstract       string   `json:"Abstract"`
	AbstractSource string   `json:"AbstractSource"`
	AbstractURL    string   `json:"AbstractURL"`
	AbstractText   string   `json:"AbstractText"`
	Heading        string   `json:"Heading"`
	Image          string   `json:"Image"`
	Infobox        string   `json:"Infobox"`
	Answer         string   `json:"Answer"`
	AnswerType     string   `json:"AnswerType"`
	Definition     string   `json:"Definition"`
	DefinitionSource string  `json:"DefinitionSource"`
	DefinitionURL    string `json:"DefinitionURL"`
	RelatedTopics    []map[string]any `json:"RelatedTopics"`
	Results         []map[string]any `json:"Results"`
	Type            string `json:"Type"`
}

// NewWebSearchTool creates a new web search tool
func NewWebSearchTool(timeoutSeconds int, maxResults int) *WebSearchTool {
	if maxResults <= 0 {
		maxResults = 10 // Default max results
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	return &WebSearchTool{
		timeout:     timeout,
		maxResults:  maxResults,
		userAgent:   "Machinus-Agent/1.0",
		apiEndpoint: "https://api.duckduckgo.com/",
	}
}

func (t *WebSearchTool) Name() string {
	return "websearch"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for current information using DuckDuckGo API."
}

func (t *WebSearchTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{Input: map[string]any{"query": "golang tutorial"}, Description: "Search for Go tutorials"},
		{Input: map[string]any{"query": "docker install"}, Description: "Search Docker installation"},
	}
}

func (t *WebSearchTool) WhenToUse() string {
	return "Use when you need current information from the internet."
}

func (t *WebSearchTool) ChainsWith() []string {
	return []string{"http", "write_file", "read_file"}
}

func (t *WebSearchTool) ValidateArgs(args map[string]any) error {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return fmt.Errorf("missing or invalid 'query' argument")
	}
	return nil
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	// Parse arguments
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return types.ToolResult{
			Success: false,
			Error:   "missing or invalid 'query' argument",
		}, nil
	}

	// Get max results (use tool default or override)
	maxResults := t.maxResults
	if mr, ok := args["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	}

	// Encode query
	encodedQuery := url.QueryEscape(query)

	// Build API URL with parameters
	apiURL := fmt.Sprintf("%s?q=%s&format=json&no_html=1&skip_disambig=0",
		t.apiEndpoint, encodedQuery)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	// Set headers
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("Accept", "application/json")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: t.timeout,
	}

	// Make request
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return types.ToolResult{
			Success:      false,
			Error:        fmt.Sprintf("search request failed: %v", err),
			FailureType:  types.FailureTypeSoft,
			Retryable:    true,
			Alternatives: []string{"http", "browser"},
		}, nil
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("search API returned status %d", resp.StatusCode),
			Data: map[string]any{
				"status_code": resp.StatusCode,
				"status":      resp.Status,
			},
		}, nil
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}, nil
	}

	// Parse JSON response
	var result DuckDuckGoInstantAnswer
	if err := json.Unmarshal(body, &result); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse response: %v", err),
		}, nil
	}

	// Format output
	output := fmt.Sprintf("Web Search: %s\n", query)
	output += fmt.Sprintf("Search Duration: %v\n\n", duration)

	// Add instant answer if available
	if result.Answer != "" {
		output += fmt.Sprintf("Answer: %s\n\n", result.Answer)
	}

	if result.Abstract != "" {
		output += fmt.Sprintf("Abstract: %s\n", result.Abstract)
		if result.AbstractSource != "" {
			output += fmt.Sprintf("Source: %s\n", result.AbstractSource)
		}
		if result.AbstractURL != "" {
			output += fmt.Sprintf("URL: %s\n", result.AbstractURL)
		}
		output += "\n"
	}

	if result.Definition != "" {
		output += fmt.Sprintf("Definition: %s\n", result.Definition)
		if result.DefinitionSource != "" {
			output += fmt.Sprintf("Source: %s\n", result.DefinitionSource)
		}
		if result.DefinitionURL != "" {
			output += fmt.Sprintf("URL: %s\n", result.DefinitionURL)
		}
		output += "\n"
	}

	// Add related topics/results
	topicCount := 0
	if len(result.RelatedTopics) > 0 {
		output += "Related Results:\n"
		for i, topic := range result.RelatedTopics {
			if i >= maxResults {
				break
			}

			// Extract information from topic
			if text, ok := topic["Text"].(string); ok && text != "" {
				output += fmt.Sprintf("\n%d. %s\n", i+1, text)
				topicCount++

				// Add first URL if available
				if firstURL, ok := topic["FirstURL"].(string); ok && firstURL != "" {
					output += fmt.Sprintf("   URL: %s\n", firstURL)
				}
			}
		}
	}

	if len(result.Results) > 0 && topicCount == 0 {
		output += "Results:\n"
		for i, res := range result.Results {
			if i >= maxResults {
				break
			}

			if text, ok := res["Text"].(string); ok && text != "" {
				output += fmt.Sprintf("\n%d. %s\n", i+1, text)
				topicCount++

				if firstURL, ok := res["FirstURL"].(string); ok && firstURL != "" {
					output += fmt.Sprintf("   URL: %s\n", firstURL)
				}
			}
		}
	}

	if topicCount == 0 && result.Answer == "" && result.Abstract == "" && result.Definition == "" {
		output += "No detailed results found. Try a different search query.\n"
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"query":         query,
			"answer":        result.Answer,
			"abstract":      result.Abstract,
			"abstract_url":  result.AbstractURL,
			"definition":    result.Definition,
			"definition_url": result.DefinitionURL,
			"heading":       result.Heading,
			"image":         result.Image,
			"type":          result.Type,
			"related_topics": result.RelatedTopics,
			"results":        result.Results,
			"duration_ms":    duration.Milliseconds(),
			"count":          topicCount,
		},
	}, nil
}
