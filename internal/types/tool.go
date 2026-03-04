package types

import "context"

// FailureType represents the category of failure
type FailureType string

const (
	FailureTypeHard      FailureType = "hard"      // Impossible to succeed (file not found, auth failed)
	FailureTypeSoft      FailureType = "soft"      // Temporary (network timeout, rate limit)
	FailureTypePartial   FailureType = "partial"   // Worked but not fully (copied 9/10 files)
	FailureTypeAmbiguous FailureType = "ambiguous" // Unclear what went wrong
)

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Output  string      `json:"output"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`

	// Error recovery metadata
	FailureType   FailureType `json:"failure_type,omitempty"`   // Type of failure
	Retryable     bool        `json:"retryable,omitempty"`      // Whether retrying could succeed
	Alternatives  []string    `json:"alternatives,omitempty"`   // Suggested alternative tools
	CanPartial    bool        `json:"can_partial,omitempty"`    // Can return partial results
	Progress      float64     `json:"progress,omitempty"`       // 0.0 to 1.0 for partial progress
}

// ToolExample represents an example usage of a tool
type ToolExample struct {
	Input    map[string]any `json:"input"`     // Example arguments
	Description string      `json:"description"` // What this example does
}

// Tool represents an executable tool interface
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]any) (ToolResult, error)
	ValidateArgs(args map[string]any) error

	// Metadata methods (optional - return empty/nil if not implemented)
	Examples() []ToolExample   // Example usages for the LLM
	WhenToUse() string         // When this tool should be used
	ChainsWith() []string      // Tools that typically follow this tool
}

// Default implementations for backward compatibility
type ToolBase struct{}

func (ToolBase) Examples() []ToolExample { return nil }
func (ToolBase) WhenToUse() string       { return "" }
func (ToolBase) ChainsWith() []string    { return nil }

// PlanStep represents a single step in the execution plan
type PlanStep struct {
	Tool           string                 `json:"tool"`
	Args           map[string]any         `json:"args"`
	RequireConfirm bool                   `json:"require_confirm"`
	Description    string                 `json:"description,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Plan represents a structured execution plan
type Plan struct {
	Steps       []PlanStep `json:"steps"`
	Description string     `json:"description"`
	Reasoning   string     `json:"reasoning,omitempty"`
}

// ExecutionAttempt represents a single execution attempt (for transparency)
type ExecutionAttempt struct {
	Tool          string                 `json:"tool"`
	Args          map[string]any         `json:"args"`
	Error         string                 `json:"error,omitempty"`
	FailureType   FailureType            `json:"failure_type,omitempty"`
	Duration      int64                  `json:"duration_ms"` // milliseconds
	AttemptNumber int                    `json:"attempt_number"`
	PartialResult interface{}            `json:"partial_result,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionHistory tracks all attempts for transparency
type ExecutionHistory struct {
	ToolName string             `json:"tool_name"`
	Attempts []ExecutionAttempt `json:"attempts"`
	Success  bool               `json:"success"`
	Duration int64              `json:"total_duration_ms"`
}

// RetryPolicy defines how to retry failed tool executions
type RetryPolicy struct {
	MaxAttempts int                    `json:"max_attempts"`
	InitialBackoff int64               `json:"initial_backoff_ms"` // milliseconds
	MaxBackoff int64                   `json:"max_backoff_ms"`     // milliseconds
	ShouldRetry func(error) bool       `json:"-"`                  // Custom retry logic
	OnRetry    func(ExecutionAttempt) (bool, map[string]any) `json:"-"` // Returns (shouldContinue, newArgs)
}

// DefaultRetryPolicy returns a standard retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 1000,  // 1 second
		MaxBackoff:     10000, // 10 seconds
		ShouldRetry: func(err error) bool {
			// Retry by default unless error is explicit about not retryable
			return err == nil || !containsNonRetryableKeyword(err.Error())
		},
	}
}

// containsNonRetryableKeyword checks if error message indicates permanent failure
func containsNonRetryableKeyword(msg string) bool {
	nonRetryable := []string{
		"not found", "404", "authentication failed", "unauthorized",
		"permission denied", "invalid argument", "syntax error",
	}
	lowerMsg := toLower(msg)
	for _, keyword := range nonRetryable {
		if contains(lowerMsg, keyword) {
			return true
		}
	}
	return false
}

// Helper functions for string operations
func toLower(s string) string {
	// Simple lowercase conversion
	if len(s) == 0 {
		return s
	}
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
