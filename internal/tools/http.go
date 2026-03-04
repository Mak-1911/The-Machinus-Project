package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// HTTPTool makes HTTP requests
type HTTPTool struct {
	timeout   time.Duration
	maxSize   int64 // Maximum response size in bytes
	userAgent string
}

// NewHTTPTool creates a new HTTP tool
func NewHTTPTool(timeoutSeconds int, maxResponseSizeMB int) *HTTPTool {
	maxSize := int64(maxResponseSizeMB) * 1024 * 1024
	timeout := time.Duration(timeoutSeconds) * time.Second

	return &HTTPTool{
		timeout:   timeout,
		maxSize:   maxSize,
		userAgent: "Machinus-Agent/1.0",
	}
}

func (t *HTTPTool) Name() string {
	return "http"
}

func (t *HTTPTool) Description() string {
	return "Make HTTP requests to APIs, web services, and fetch web pages. Supports GET, POST, PUT, DELETE, PATCH methods with custom headers and JSON bodies."
}

func (t *HTTPTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"url":    "https://api.github.com/repos/golang/go",
				"method": "GET",
			},
			Description: "Fetch information about a GitHub repository",
		},
		{
			Input: map[string]any{
				"url":    "https://jsonplaceholder.typicode.com/posts",
				"method": "POST",
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
				"body": map[string]any{
					"title":  "foo",
					"body":   "bar",
					"userId": 1,
				},
			},
			Description: "Create a new post via POST request with JSON body",
		},
		{
			Input: map[string]any{
				"url":    "https://api.example.com/data",
				"method": "GET",
				"headers": map[string]string{
					"Authorization": "Bearer YOUR_TOKEN",
				},
			},
			Description: "Make authenticated API request with bearer token",
		},
		{
			Input: map[string]any{
				"url":         "https://example.com/search",
				"method":      "GET",
				"query_params": map[string]string{
					"q":     "golang",
					"page":  "1",
					"limit": "10",
				},
			},
			Description: "Make GET request with query parameters",
		},
	}
}

func (t *HTTPTool) WhenToUse() string {
	return "Use to interact with HTTP APIs, fetch web pages, download data, send webhooks, or test HTTP endpoints. Supports all common HTTP methods and custom headers."
}

func (t *HTTPTool) ChainsWith() []string {
	return []string{"write_file", "read_file"}
}

func (t *HTTPTool) ValidateArgs(args map[string]any) error {
	url, ok := args["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("missing or invalid 'url' argument")
	}

	// Validate URL format
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}

	method, ok := args["method"].(string)
	if !ok || method == "" {
		return fmt.Errorf("missing or invalid 'method' argument")
	}

	// Validate method
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	if !validMethods[strings.ToUpper(method)] {
		return fmt.Errorf("invalid method: %s", method)
	}

	return nil
}

func (t *HTTPTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	// Parse arguments
	url, _ := args["url"].(string)
	method := strings.ToUpper(args["method"].(string))

	// Build query parameters
	if queryParams, ok := args["query_params"].(map[string]any); ok {
		queryString := ""
		for key, value := range queryParams {
			if queryString != "" {
				queryString += "&"
			}
			queryString += fmt.Sprintf("%s=%v", key, value)
		}
		if queryString != "" {
			if strings.Contains(url, "?") {
				url += "&" + queryString
			} else {
				url += "?" + queryString
			}
		}
	}

	// Prepare request body
	var bodyReader io.Reader
	if body, ok := args["body"]; ok && body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal body: %v", err),
			}, nil
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	// Set headers
	req.Header.Set("User-Agent", t.userAgent)

	// Set default content type for POST/PUT/PATCH if body exists
	if bodyReader != nil {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// Set custom headers
	if headers, ok := args["headers"].(map[string]any); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: t.timeout,
		// Follow redirects (default is 10)
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	// Make request
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		// Analyze error type for retry logic
		errStr := err.Error()
		failureType := types.FailureTypeSoft
		retryable := true
		alternatives := []string{}

		// Check for timeout
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
			failureType = types.FailureTypeSoft
			retryable = true
			alternatives = []string{"browser", "shell"}
		} else if strings.Contains(errStr, "connection refused") {
			failureType = types.FailureTypeHard
			retryable = false
		}

		return types.ToolResult{
			Success:      false,
			Error:        fmt.Sprintf("request failed: %v", err),
			FailureType:  failureType,
			Retryable:    retryable,
			Alternatives: alternatives,
		}, nil
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// Limit response size
	limitedReader := io.LimitReader(resp.Body, t.maxSize)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}, nil
	}

	// Check if we hit the size limit
	if int64(len(respBody)) >= t.maxSize {
		return types.ToolResult{
			Success:      false,
			Error:        fmt.Sprintf("response too large (exceeds %d MB)", t.maxSize/(1024*1024)),
			FailureType:  types.FailureTypePartial,
			Retryable:    false,
			CanPartial:   true,
			Progress:     1.0, // We got what we could within the limit
			Alternatives: []string{"browser", "shell"},
			Data: map[string]any{
				"partial_body":  string(respBody),
				"actual_size":   len(respBody),
				"max_size":      t.maxSize,
				"size_exceeded": true,
			},
		}, nil
	}

	// Format response headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = strings.Join(values, ", ")
		}
	}

	// Build output
	output := fmt.Sprintf("HTTP %s %s\n", method, url)
	output += fmt.Sprintf("Status: %d %s\n", resp.StatusCode, resp.Status)
	output += fmt.Sprintf("Duration: %v\n", duration)

	if len(headers) > 0 {
		output += "\nHeaders:\n"
		for key, value := range headers {
			output += fmt.Sprintf("  %s: %s\n", key, value)
		}
	}

	output += fmt.Sprintf("\nResponse Body (%d bytes):\n", len(respBody))

	// Try to format JSON responses
	var formattedBody string
	if resp.Header.Get("Content-Type") != "" &&
		strings.Contains(resp.Header.Get("Content-Type"), "json") {
		var jsonBuf any
		if err := json.Unmarshal(respBody, &jsonBuf); err == nil {
			if formatted, err := json.MarshalIndent(jsonBuf, "", "  "); err == nil {
				formattedBody = string(formatted)
			} else {
				formattedBody = string(respBody)
			}
		} else {
			formattedBody = string(respBody)
		}
	} else {
		formattedBody = string(respBody)
	}

	output += formattedBody

	// Determine success based on status code
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	result := types.ToolResult{
		Success: success,
		Output:  output,
		Data: map[string]any{
			"url":         url,
			"method":      method,
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"headers":     headers,
			"body":        string(respBody),
			"duration_ms": duration.Milliseconds(),
		},
	}

	// Add error recovery metadata for failed requests
	if !success {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// 4xx errors - client errors, typically not retryable
			result.FailureType = types.FailureTypeHard
			result.Retryable = false

			// Specific error codes
			if resp.StatusCode == 404 {
				result.Alternatives = []string{"browser", "search"}
			} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
				result.Alternatives = []string{} // No alternative for auth issues
			} else {
				result.Alternatives = []string{"browser"}
			}
		} else if resp.StatusCode >= 500 {
			// 5xx errors - server errors, potentially retryable
			result.FailureType = types.FailureTypeSoft
			result.Retryable = true
			result.Alternatives = []string{"browser", "shell"}
		}
	}

	return result, nil
}
