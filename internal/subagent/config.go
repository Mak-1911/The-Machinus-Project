package subagent

import (
	"fmt"
	"time"
)

type SubagentConfig struct {
	ID 				string
	SystemPrompt 	string
	Tools 			[]string
	Task 			string
	Timeout 		time.Duration
	MaxSteps		int
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig(task string) SubagentConfig {
	return SubagentConfig{
		ID: 			generateID(),
		Task: 			task,
		Timeout: 		2 * time.Minute,
		MaxSteps:		20,
		Tools:			[]string{},
		SystemPrompt: 	"",
	}
}

func generateID() string {
	return fmt.Sprintf("sub=%d", time.Now().UnixNano())
}
