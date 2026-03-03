package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
	"github.com/playwright-community/playwright-go"
)

// BrowserTool handles browser automation using Playwright
type BrowserTool struct {
	browser  playwright.Browser
	context  playwright.BrowserContext
	page     playwright.Page
	timeout  time.Duration
	headless bool
}

// NewBrowserTool creates a new browser automation tool
func NewBrowserTool() *BrowserTool {
	return &BrowserTool{
		timeout:  30 * time.Second,
		headless: true,
	}
}

func (t *BrowserTool) Name() string {
	return "browser"
}

func (t *BrowserTool) Description() string {
	return "Automate web browsers - navigate, click, fill forms, extract data, take screenshots. Supports any browser-based task."
}

func (t *BrowserTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "goto", "url": "https://example.com"},
					{"action": "screenshot", "path": "screenshot.png"},
				},
			},
			Description: "Navigate to a website and take a screenshot",
		},
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "goto", "url": "https://github.com"},
					{"action": "fill", "selector": "input[name='q']", "value": "playwright"},
					{"action": "click", "selector": "button[type='submit']"},
				},
			},
			Description: "Search on GitHub",
		},
		{
			Input: map[string]any{
				"actions": []map[string]any{
					{"action": "goto", "url": "https://example.com"},
					{"action": "text", "selector": "h1"},
				},
			},
			Description: "Extract text from a page element",
		},
	}
}

func (t *BrowserTool) WhenToUse() string {
	return "Use for web automation tasks: filling forms, clicking buttons, scraping data from JavaScript-heavy sites, taking screenshots, testing web applications, or any task requiring browser interaction."
}

func (t *BrowserTool) ChainsWith() []string {
	return []string{"http", "write_file", "read_file"}
}

func (t *BrowserTool) ValidateArgs(args map[string]any) error {
	actions, ok := args["actions"].([]map[string]any)
	if !ok || len(actions) == 0 {
		return fmt.Errorf("missing or invalid 'actions' argument - must be a list of actions")
	}

	// Validate each action
	for i, action := range actions {
		actionType, ok := action["action"].(string)
		if !ok || actionType == "" {
			return fmt.Errorf("action %d: missing 'action' field", i+1)
		}

		// Validate required fields for each action type
		switch actionType {
		case "goto":
			if _, ok := action["url"].(string); !ok {
				return fmt.Errorf("action %d: 'goto' requires 'url'", i+1)
			}
		case "click", "fill", "text", "wait":
			if _, ok := action["selector"].(string); !ok {
				return fmt.Errorf("action %d: '%s' requires 'selector'", i+1, actionType)
			}
		case "screenshot":
			if _, ok := action["path"].(string); !ok {
				return fmt.Errorf("action %d: 'screenshot' requires 'path'", i+1)
			}
		}
	}

	return nil
}

func (t *BrowserTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	// Parse actions with better type handling
	var actions []map[string]any

	switch v := args["actions"].(type) {
	case []map[string]any:
		actions = v
	case []interface{}:
		// Convert []interface{} to []map[string]any
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Convert map[string]interface{} to map[string]any
				converted := make(map[string]any)
				for k, val := range itemMap {
					converted[k] = val
				}
				actions = append(actions, converted)
			}
		}
	}

	if len(actions) == 0 {
		return types.ToolResult{
			Success: false,
			Error:   "no actions provided - actions array is empty or invalid format",
		}, nil
	}

	// Initialize Playwright
	pw, err := playwright.Run()
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to launch playwright: %v", err),
		}, nil
	}
	defer pw.Stop()

	// Launch browser
	opts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(t.headless),
	}
	browser, err := pw.Chromium.Launch(opts)
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to launch browser: %v", err),
		}, nil
	}
	defer browser.Close()

	// Create context
	browserContext, err := browser.NewContext()
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create context: %v", err),
		}, nil
	}
	defer browserContext.Close()
	t.context = browserContext

	// Create page
	page, err := browserContext.NewPage()
	if err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create page: %v", err),
		}, nil
	}
	t.page = page

	// Set default timeout
	page.SetDefaultTimeout(float64(t.timeout.Milliseconds()))

	// Execute actions
	output := fmt.Sprintf("Executing %d browser actions...\n\n", len(actions))
	results := make([]string, 0, len(actions))

	for i, action := range actions {
		actionType, _ := action["action"].(string)

		output += fmt.Sprintf("Step %d: %s", i+1, actionType)

		result, err := t.executeAction(action)
		if err != nil {
			output += fmt.Sprintf(" ❌\n   Error: %v\n", err)
			results = append(results, fmt.Sprintf("Step %d (%s): FAILED - %v", i+1, actionType, err))
			continue
		}

		output += fmt.Sprintf(" ✓\n")
		if result != "" {
			output += fmt.Sprintf("   → %s\n", truncate(result, 200))
			output += "\n"
		}
		results = append(results, fmt.Sprintf("Step %d (%s): SUCCESS", i+1, actionType))
	}

	output += fmt.Sprintf("\n✓ Browser automation completed\n")

	return types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]any{
			"actions": len(actions),
			"results": results,
		},
	}, nil
}

// executeAction executes a single browser action
func (t *BrowserTool) executeAction(action map[string]any) (string, error) {
	actionType, _ := action["action"].(string)

	switch actionType {
	case "goto":
		url, _ := action["url"].(string)
		if _, err := t.page.Goto(url); err != nil {
			return "", fmt.Errorf("failed to navigate: %w", err)
		}
		// Wait for page to fully render (for SPAs)
		if err := t.page.WaitForLoadState(); err != nil {
			// Don't fail if wait fails, just continue
		}
		// Additional wait for JavaScript rendering
		time.Sleep(500 * time.Millisecond)
		return fmt.Sprintf("Navigated to %s", url), nil

	case "click":
		selector, _ := action["selector"].(string)
		if err := t.page.Click(selector); err != nil {
			return "", fmt.Errorf("failed to click: %w", err)
		}
		return fmt.Sprintf("Clicked %s", selector), nil

	case "fill":
		selector, _ := action["selector"].(string)
		value, _ := action["value"].(string)
		if err := t.page.Fill(selector, value); err != nil {
			return "", fmt.Errorf("failed to fill: %w", err)
		}
		return fmt.Sprintf("Filled %s with '%s'", selector, value), nil

	case "text":
		selector, _ := action["selector"].(string)
		textContent, err := t.page.TextContent(selector)
		if err != nil {
			return "", fmt.Errorf("failed to get text: %w", err)
		}
		return fmt.Sprintf("Text: %s", truncate(textContent, 100)), nil

	case "screenshot":
		path, _ := action["path"].(string)
		fullPage, _ := action["full_page"].(bool)
		if fullPage {
			if _, err := t.page.Screenshot(playwright.PageScreenshotOptions{
				Path: playwright.String(path),
				FullPage: playwright.Bool(true),
			}); err != nil {
				return "", fmt.Errorf("failed to take screenshot: %w", err)
			}
		} else {
			if _, err := t.page.Screenshot(playwright.PageScreenshotOptions{
				Path: playwright.String(path),
			}); err != nil {
				return "", fmt.Errorf("failed to take screenshot: %w", err)
			}
		}
		return fmt.Sprintf("Screenshot saved to %s", path), nil

	case "wait":
		selector, _ := action["selector"].(string)
		if _, err := t.page.WaitForSelector(selector); err != nil {
			return "", fmt.Errorf("failed to wait for selector: %w", err)
		}
		return fmt.Sprintf("Waited for %s", selector), nil

	case "wait_for_navigation":
		if err := t.page.WaitForLoadState(); err != nil {
			return "", fmt.Errorf("failed to wait for navigation: %w", err)
		}
		return "Waited for page load", nil

	case "evaluate":
		script, _ := action["script"].(string)
		result, err := t.page.Evaluate(script)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate script: %w", err)
		}
		return fmt.Sprintf("Result: %v", result), nil

	case "html":
		selector, _ := action["selector"].(string)
		html, err := t.page.InnerHTML(selector)
		if err != nil {
			return "", fmt.Errorf("failed to get HTML: %w", err)
		}
		return fmt.Sprintf("HTML: %s", truncate(html, 200)), nil

	case "url":
		url := t.page.URL()
		return fmt.Sprintf("Current URL: %s", url), nil

	case "title":
		title, err := t.page.Title()
		if err != nil {
			return "", fmt.Errorf("failed to get title: %w", err)
		}
		return fmt.Sprintf("Page title: %s", title), nil

	default:
		return "", fmt.Errorf("unknown action: %s", actionType)
	}
}

func (t *BrowserTool) Close() error {
	var lastErr error
	if t.page != nil {
		if err := t.page.Close(); err != nil {
			lastErr = err
		}
	}
	if t.context != nil {
		if err := t.context.Close(); err != nil {
			lastErr = err
		}
	}
	if t.browser != nil {
		if err := t.browser.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
