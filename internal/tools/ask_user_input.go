// Package tools provides the AskUserInput tool for prompting users during execution.
package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// UserInputRequest represents a request for user input
type UserInputRequest struct {
	ID         string
	Message    string
	Placeholder string
	Default    string
	Options    []string
}

// UserInputResponse represents the user's response
type UserInputResponse struct {
	Input string
	Err   error
}

// InputCallback is called when the tool needs to request user input.
// It should return the user's response or an error.
type InputCallback func(req UserInputRequest) (string, error)

// AskUserInputTool prompts the user for input during execution.
// This allows the AI to ask clarifying questions or request additional information.
type AskUserInputTool struct {
	mu       sync.RWMutex
	callback InputCallback
	timeout  time.Duration
}

// NewAskUserInputTool creates a new AskUserInput tool.
func NewAskUserInputTool(timeoutSeconds int) *AskUserInputTool {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute // Default 5 minutes
	}
	return &AskUserInputTool{
		timeout: timeout,
	}
}

// SetCallback sets the callback function for handling user input requests.
func (t *AskUserInputTool) SetCallback(cb InputCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callback = cb
}

// getCallback returns the current callback in a thread-safe manner.
func (t *AskUserInputTool) getCallback() InputCallback {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.callback
}

func (t *AskUserInputTool) Name() string {
	return "ask_user_input"
}

func (t *AskUserInputTool) Description() string {
	return "Prompt the user for input during execution. Use this when you need additional information, " +
		"clarification on a request, or confirmation before proceeding with an action."
}

func (t *AskUserInputTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{
				"message":    "What would you like to name the new file?",
				"placeholder": "filename.txt",
			},
			Description: "Ask user for a filename",
		},
		{
			Input: map[string]any{
				"message": "Which framework would you like to use?",
				"options": []string{"React", "Vue", "Svelte", "Angular"},
			},
			Description: "Ask user to choose from options",
		},
		{
			Input: map[string]any{
				"message": "Enter your API key:",
				"default": "",
			},
			Description: "Ask for sensitive input with no default",
		},
	}
}

func (t *AskUserInputTool) WhenToUse() string {
	return "Use when you need to: 1) Get clarification on an ambiguous request, " +
		"2) Request missing required information, 3) Get user preference between multiple valid options, " +
		"4) Request confirmation for destructive operations. " +
		"Only use when the information cannot reasonably be inferred."
}

func (t *AskUserInputTool) ChainsWith() []string {
	return []string{"write_file", "bash", "read_file"}
}

func (t *AskUserInputTool) ValidateArgs(args map[string]any) error {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return fmt.Errorf("missing or invalid 'message' argument")
	}

	// Validate options if provided
	if opts, ok := args["options"].([]string); ok {
		if len(opts) == 0 {
			return fmt.Errorf("'options' cannot be empty")
		}
	}

	return nil
}

func (t *AskUserInputTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	// Parse arguments
	message, _ := args["message"].(string)
	placeholder, _ := args["placeholder"].(string)
	defaultVal, _ := args["default"].(string)
	options, _ := args["options"].([]string)

	if placeholder == "" {
		placeholder = "Enter your response..."
	}

	// Get the callback for handling input
	callback := t.getCallback()
	if callback == nil {
		return types.ToolResult{
			Success: false,
			Error:   "user input is not available in this context",
			Data: map[string]any{
				"message": message,
			},
		}, nil
	}

	// Create a channel for the response
	responseChan := make(chan UserInputResponse, 1)

	// Create request ID
	requestID := fmt.Sprintf("input-%d", time.Now().UnixNano())

	// Call the callback in a goroutine
	go func() {
		input, err := callback(UserInputRequest{
			ID:         requestID,
			Message:    message,
			Placeholder: placeholder,
			Default:    defaultVal,
		})
		responseChan <- UserInputResponse{Input: input, Err: err}
	}()

	// Wait for response or timeout
	select {
	case <-ctx.Done():
		return types.ToolResult{
			Success: false,
			Error:   "request cancelled by context",
			Data: map[string]any{
				"message": message,
			},
		}, nil

	case <-time.After(t.timeout):
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("user input timed out after %v", t.timeout),
			Data: map[string]any{
				"message": message,
			},
		}, nil

	case response := <-responseChan:
		if response.Err != nil {
			return types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to get user input: %v", response.Err),
				Data: map[string]any{
					"message": message,
				},
			}, nil
		}

		// Validate against options if provided
		if len(options) > 0 {
			valid := false
			for _, opt := range options {
				if response.Input == opt {
					valid = true
					break
				}
			}
			if !valid && response.Input != "" {
				return types.ToolResult{
					Success: false,
					Error:   fmt.Sprintf("input must be one of: %v", options),
					Data: map[string]any{
						"message":  message,
						"input":    response.Input,
						"options":  options,
					},
				}, nil
			}
		}

		output := fmt.Sprintf("User input received: %s", response.Input)
		if len(options) > 0 {
			output = fmt.Sprintf("User selected: %s", response.Input)
		}

		return types.ToolResult{
			Success: true,
			Output:  output,
			Data: map[string]any{
				"message": message,
				"input":   response.Input,
				"options": options,
			},
		}, nil
	}
}
