// Package commands provides command types for the UI.
package commands

import "time"

// Command represents a slash command.
type Command struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Aliases     []string          `json:"aliases,omitempty"`
	Args        []CommandArg      `json:"args,omitempty"`
	Handler     string            `json:"handler"` // Name of the handler function
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CommandArg represents a command argument.
type CommandArg struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// Argument is an alias for CommandArg for compatibility.
type Argument = CommandArg

// CommandContext represents the context when a command is executed.
type CommandContext struct {
	SessionID string            `json:"session_id"`
	Args      map[string]string `json:"args"`
	Timestamp time.Time         `json:"timestamp"`
}

// CommandResult represents the result of a command execution.
type CommandResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CustomCommand represents a user-defined command.
type CustomCommand struct {
	Command
	UserDefined bool   `json:"user_defined"`
	ContentArg  string `json:"content,omitempty"`
}

// Content returns the custom command content.
func (c *CustomCommand) Content() string {
	if c.ContentArg != "" {
		return c.ContentArg
	}
	return c.Metadata["content"]
}

// Arguments returns the custom command arguments.
func (c *CustomCommand) Arguments() []CommandArg {
	return c.Args
}

// MCPPrompt represents an MCP prompt.
type MCPPrompt struct {
	Name        string       `json:"name"`
	TitleArg    string       `json:"title,omitempty"`
	Description string       `json:"description"`
	Args        []CommandArg `json:"args,omitempty"`
	PromptIDArg string       `json:"prompt_id,omitempty"`
	ClientID    string       `json:"client_id,omitempty"`
}

// Title returns the MCP prompt title.
func (m *MCPPrompt) Title() string {
	if m.TitleArg != "" {
		return m.TitleArg
	}
	return m.Description
}

// PromptID returns the prompt ID.
func (m *MCPPrompt) PromptID() string {
	if m.PromptIDArg != "" {
		return m.PromptIDArg
	}
	return m.Name
}

// Built-in commands
const (
	CommandHelp      = "help"
	CommandClear     = "clear"
	CommandQuit      = "quit"
	CommandSessions  = "sessions"
	CommandModels    = "models"
	CommandApiKey    = "api_key"
	CommandOAuth     = "oauth"
	CommandReasoning = "reasoning"
)

// DefaultCommands returns the default slash commands.
func DefaultCommands() []Command {
	return []Command{
		{
			ID:          CommandHelp,
			Name:        "help",
			Description: "Show help information",
			Handler:     "help",
		},
		{
			ID:          CommandClear,
			Name:        "clear",
			Description: "Clear the screen",
			Aliases:     []string{"cls"},
			Handler:     "clear",
		},
		{
			ID:          CommandQuit,
			Name:        "quit",
			Description: "Exit the application",
			Aliases:     []string{"exit", "q"},
			Handler:     "quit",
		},
		{
			ID:          CommandSessions,
			Name:        "sessions",
			Description: "Manage sessions",
			Handler:     "sessions",
		},
		{
			ID:          CommandModels,
			Name:        "models",
			Description: "Select a model",
			Handler:     "models",
		},
		{
			ID:          CommandApiKey,
			Name:        "api_key",
			Description: "Set API key",
			Handler:     "api_key",
		},
		{
			ID:          CommandOAuth,
			Name:        "oauth",
			Description: "Authenticate with OAuth",
			Handler:     "oauth",
		},
		{
			ID:          CommandReasoning,
			Name:        "reasoning",
			Description: "Toggle reasoning display",
			Handler:     "reasoning",
		},
	}
}

// LoadCustomCommands loads custom commands.
func LoadCustomCommands(cfg interface{}) ([]CustomCommand, error) {
	return nil, nil
}

// LoadMCPPrompts loads MCP prompts.
func LoadMCPPrompts(cfg interface{}) ([]Command, error) {
	return nil, nil
}

// GetMCPPrompt gets an MCP prompt by name.
func GetMCPPrompt(cfg any, clientID, promptID string, arguments map[string]string) (string, error) {
	return "", nil // Placeholder
}
