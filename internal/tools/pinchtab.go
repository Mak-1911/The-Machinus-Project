package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// PinchTabTool integrates with PinchTab HTTP API
type PinchTabTool struct {
	baseURL    string
	httpClient *http.Client
	autoStart  bool
	checked    bool  // Track if we've checked for running
	running    bool  // Cache running state
	mu         sync.Mutex // Protects checked/running
}

// NewPinchTabTool creates a new PinchTab tool
func NewPinchTabTool(baseURL string) *PinchTabTool {
	if baseURL == "" {
		baseURL = "http://localhost:9867"
	}

	return &PinchTabTool{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		autoStart: true,
		checked: false,  // Don't check on init - lazy load
		running: false,
	}
}

// isPinchTabRunning checks if PinchTab server is accessible
func (t *PinchTabTool) isPinchTabRunning(ctx context.Context) bool {
	// Try to list instances - this endpoint exists and returns 200 even if empty
	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/instances", nil)
	if err != nil {
		return false
	}

	// Use shorter timeout for health check
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// PinchTab returns 200 OK even with no instances
	return resp.StatusCode == http.StatusOK
}

// startPinchTab attempts to start the PinchTab server
func (t *PinchTabTool) startPinchTab(ctx context.Context) error {
	// Check if pinchtab command exists
	_, err := exec.LookPath("pinchtab")
	if err != nil {
		return fmt.Errorf("pinchtab not found - install with: npm install -g pinchtab")
	}

	fmt.Printf("  → Starting pinchtab command...\n")

	// Start pinchtab in background
	cmd := exec.CommandContext(ctx, "pinchtab")
	if runtime.GOOS == "windows" {
		// On Windows, use start to detach process
		cmd = exec.Command("cmd", "/c", "start", "/b", "pinchtab")
	}

	// Set up output to avoid blocking
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pinchtab: %w", err)
	}

	// Detach the process
	go cmd.Wait()

	fmt.Printf("  → Waiting for PinchTab to be ready...\n")

	// Wait for server to be ready
	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	attempts := 0
	for {
		select {
		case <-readyCtx.Done():
			return fmt.Errorf("timeout waiting for pinchtab to start - it may have failed to start or is already running on a different port")
		case <-time.After(500 * time.Millisecond):
			attempts++
			if t.isPinchTabRunning(ctx) {
				fmt.Printf("  → PinchTab ready after %d seconds\n", attempts/2)
				return nil
			}
		}
	}
}

// ensurePinchTabRunning checks and starts PinchTab if needed (lazy, thread-safe)
func (t *PinchTabTool) ensurePinchTabRunning(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check cache first
	if t.checked && t.running {
		return nil
	}

	// Check if PinchTab is running (only check once)
	if !t.checked {
		t.checked = true
		t.running = t.isPinchTabRunning(ctx)

		if t.running {
			return nil  // Already running, no action needed
		}
	}

	// Not running and auto-start is disabled
	if !t.autoStart {
		return fmt.Errorf("pinchtab is not running at %s - start it with: pinchtab", t.baseURL)
	}

	// Try to start it
	fmt.Printf("🔄 PinchTab not detected, starting automatically...\n")

	if err := t.startPinchTab(ctx); err != nil {
		return err
	}

	t.running = true
	fmt.Printf("✅ PinchTab started successfully\n")
	return nil
}

func (t *PinchTabTool) Name() string {
	return "browser"
}

func (t *PinchTabTool) Description() string {
	return "Automate Chrome browsers via PinchTab - navigate, click, fill forms, extract text efficiently. Supports multi-instance, persistent sessions, and accessibility-based element selection."
}

func (t *PinchTabTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "create_instance", "profile": "default"},
				},
			},
			Description: "Create a new browser instance",
		},
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "navigate", "url": "https://example.com"},
				},
			},
			Description: "Navigate to a URL",
		},
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "snapshot"},
				},
			},
			Description: "Get page structure with interactive elements",
		},
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "click", "ref": "e5"},
				},
			},
			Description: "Click an element by ref",
		},
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "fill", "ref": "e3", "value": "user@example.com"},
				},
			},
			Description: "Fill a form field",
		},
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "text"},
				},
			},
			Description: "Extract text content (token-efficient)",
		},
	}
}

func (t *PinchTabTool) WhenToUse() string {
	return "Use when you need to automate web browsers: navigate websites, click buttons, fill forms, extract data, or take screenshots. Much more token-efficient than screenshots for text extraction."
}

func (t *PinchTabTool) ChainsWith() []string {
	return []string{"text", "screenshot", "snapshot"}
}

func (t *PinchTabTool) ValidateArgs(args map[string]any) error {
	actions, ok := args["actions"]
	if !ok {
		return fmt.Errorf("missing required field: actions")
	}

	_, ok = actions.([]any)
	if !ok {
		return fmt.Errorf("actions must be an array")
	}

	return nil
}

func (t *PinchTabTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	actions, ok := args["actions"].([]any)
	if !ok || len(actions) == 0 {
		return types.ToolResult{
			Success: false,
			Error:   "no actions provided",
		}, nil
	}

	// Auto-start PinchTab if not running
	if err := t.ensurePinchTabRunning(ctx); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to start PinchTab: %w", err),
		}, nil
	}

	var output string
	var results []string
	var currentInstanceID string
	var currentTabID string

	output += fmt.Sprintf("Executing %d browser action(s)...\n", len(actions))

	for i, action := range actions {
		actionMap, ok := action.(map[string]any)
		if !ok {
			results = append(results, fmt.Sprintf("Step %d: INVALID - action is not a map", i+1))
			continue
		}

		actionType, _ := actionMap["action"].(string)
		output += fmt.Sprintf("  [%d] %s", i+1, actionType)

		result, instanceID, tabID, err := t.executeAction(ctx, actionMap, currentInstanceID, currentTabID)
		if err != nil {
			output += fmt.Sprintf(" ❌\n")
			output += fmt.Sprintf("     Error: %s\n", err.Error())
			results = append(results, fmt.Sprintf("Step %d (%s): FAILED - %s", i+1, actionType, err.Error()))
		} else {
			output += fmt.Sprintf(" ✓\n")
			if result != "" {
				output += fmt.Sprintf("     → %s\n", result)
			}
			results = append(results, fmt.Sprintf("Step %d (%s): SUCCESS", i+1, actionType))

			// Update instance/tab IDs if returned
			if instanceID != "" {
				currentInstanceID = instanceID
			}
			if tabID != "" {
				currentTabID = tabID
			}
		}
	}

	output += fmt.Sprintf("\n✓ Browser automation completed\n")

	// Include instance/tab IDs in result for reference
	data := map[string]any{
		"actions": len(actions),
		"results": results,
	}
	if currentInstanceID != "" {
		data["instanceId"] = currentInstanceID
	}
	if currentTabID != "" {
		data["tabId"] = currentTabID
	}

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data:    data,
	}, nil
}

// executeAction executes a single PinchTab action
func (t *PinchTabTool) executeAction(ctx context.Context, action map[string]any, instanceID, tabID string) (string, string, string, error) {
	actionType, _ := action["action"].(string)

	switch actionType {
	case "navigate", "goto":
		return t.navigate(ctx, action)

	case "snapshot":
		return t.snapshot(ctx)

	case "click":
		return t.click(ctx, action)

	case "fill":
		return t.fill(ctx, action)

	case "text":
		return t.extractText(ctx)

	case "screenshot":
		return t.screenshot(ctx, action)

	default:
		return "", "", "", fmt.Errorf("unknown action: %s (supported: navigate, snapshot, click, fill, text, screenshot)", actionType)
	}
}

// navigate navigates to a URL

// navigate navigates to a URL
func (t *PinchTabTool) navigate(ctx context.Context, action map[string]any) (string, string, string, error) {
	url, _ := action["url"].(string)
	if url == "" {
		return "", "", "", fmt.Errorf("url is required")
	}

	reqBody := map[string]string{
		"url": url,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/navigate", bytes.NewReader(body))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("navigation failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("navigation failed: %s", string(respBody))
	}

	var result struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Sprintf("Navigated to %s", url), "", "", nil
	}

	return fmt.Sprintf("Navigated to %s (title: %s)", url, result.Title), "", "", nil
}

// snapshot gets the page structure
func (t *PinchTabTool) snapshot(ctx context.Context) (string, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/snapshot", nil)
	if err != nil {
		return "", "", "", err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("snapshot failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("snapshot failed: %s", string(respBody))
	}

	var result struct {
		Count int              `json:"count"`
		Nodes []map[string]any `json:"nodes"`
		Title string           `json:"title"`
		URL   string           `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", "", err
	}

	// Return a summary of the snapshot
	summary := fmt.Sprintf("Page snapshot: %d interactive elements", result.Count)
	if len(result.Nodes) > 0 {
		// Show first few elements as preview
		preview := ""
		for i, node := range result.Nodes {
			if i >= 5 {
				break
			}
			if ref, ok := node["ref"].(string); ok {
				if name, ok := node["name"].(string); ok {
					role := ""
					if r, ok := node["role"].(string); ok {
						role = fmt.Sprintf("[%s] ", r)
					}
					preview += fmt.Sprintf("\n     [%s] %s%s", ref, role, truncate(name, 50))
				}
			}
		}
		if result.Count > 5 {
			preview += fmt.Sprintf("\n     ... and %d more", result.Count-5)
		}
		summary += preview
	}

	return summary, "", "", nil
}

// click clicks an element
func (t *PinchTabTool) click(ctx context.Context, action map[string]any) (string, string, string, error) {
	ref, _ := action["ref"].(string)
	if ref == "" {
		return "", "", "", fmt.Errorf("ref is required for click")
	}

	reqBody := map[string]string{
		"kind": "click",
		"ref":  ref,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/action", bytes.NewReader(body))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("click failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("click failed: %s", string(respBody))
	}

	return fmt.Sprintf("Clicked element: %s", ref), "", "", nil
}

// fill fills a form field
func (t *PinchTabTool) fill(ctx context.Context, action map[string]any) (string, string, string, error) {
	ref, _ := action["ref"].(string)
	if ref == "" {
		return "", "", "", fmt.Errorf("ref is required for fill")
	}

	value, _ := action["value"].(string)
	if value == "" {
		return "", "", "", fmt.Errorf("value is required for fill")
	}

	reqBody := map[string]any{
		"kind":  "fill",
		"ref":   ref,
		"value": value,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/action", bytes.NewReader(body))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("fill failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("fill failed: %s", string(respBody))
	}

	return fmt.Sprintf("Filled %s: %s", ref, value), "", "", nil
}

// extractText extracts text from the page
func (t *PinchTabTool) extractText(ctx context.Context) (string, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/text", nil)
	if err != nil {
		return "", "", "", err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("text extraction failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("text extraction failed: %s", string(respBody))
	}

	var result struct {
		Text  string `json:"text"`
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", "", err
	}

	// Return truncated text
	return fmt.Sprintf("Page text:\n%s", truncate(result.Text, 1000)), "", "", nil
}

// screenshot takes a screenshot
func (t *PinchTabTool) screenshot(ctx context.Context, action map[string]any) (string, string, string, error) {
	path, _ := action["path"].(string)
	if path == "" {
		path = "screenshot.png"
	}

	// PinchTab returns base64 screenshot
	url := fmt.Sprintf("%s/screenshot?path=%s", t.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", "", err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("screenshot failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("screenshot failed: %s", string(respBody))
	}

	var result struct {
		Base64 string `json:"base64"`
		Format string `json:"format"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", "", err
	}

	return fmt.Sprintf("Screenshot captured (format: %s, size: %d bytes)", result.Format, len(result.Base64)/4*3), "", "", nil
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
