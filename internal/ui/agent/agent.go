// Package agent provides agent types for the UI.
package agent

import "time"

// Agent represents an agent instance.
type Agent struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Model     string            `json:"model"`
	StartedAt time.Time         `json:"started_at"`
	Status    string            `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// AgentStatus represents the status of an agent.
type AgentStatus string

const (
	AgentStatusIdle     AgentStatus = "idle"
	AgentStatusRunning  AgentStatus = "running"
	AgentStatusError    AgentStatus = "error"
	AgentStatusStopped  AgentStatus = "stopped"
)

// Capabilities represents agent capabilities.
type Capabilities struct {
	ScreenCapture bool `json:"screen_capture"`
	AudioInput    bool `json:"audio_input"`
	Clipboard     bool `json:"clipboard"`
	Notifications bool `json:"notifications"`
}

// DefaultCapabilities returns the default capabilities.
func DefaultCapabilities() Capabilities {
	return Capabilities{
		ScreenCapture: false,
		AudioInput:    false,
		Clipboard:     false,
		Notifications: false,
	}
}

// AgentParams represents agent tool parameters.
type AgentParams struct {
	Prompt string `json:"prompt"`
}

// AgentToolName is the name of the agent tool.
const AgentToolName = "agent"

// Model represents an agent model.
type Model struct {
	Name                  string
	Provider             string
	CanReason            bool
	ReasoningLevels      []string
	ReasoningEffort      string
	DefaultReasoningEffort string
	Think                bool
	ContextWindow        int
}

// ModelCfg returns model config.
func (m *Model) ModelCfg() *Model { return m }

// CatwalkCfg returns catwalk config.
func (m *Model) CatwalkCfg() *Model { return m }

// InitializePrompt returns the initialization prompt.
func InitializePrompt(cfg interface{}) (string, error) {
	return "", nil
}
