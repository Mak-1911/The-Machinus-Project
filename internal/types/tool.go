package types

import "context"

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Output  string      `json:"output"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
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
