package agent

import (
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// Task represents a user task
type Task struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	Message     string      `json:"message"`
	Plan        *types.Plan `json:"plan,omitempty"`
	Response    string      `json:"response,omitempty"` // Conversational response when no tools needed
	Status      string      `json:"status"`             // pending, planning, executing, completed, failed
	CurrentStep int         `json:"current_step"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// TaskLog represents a log entry for a task
type TaskLog struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Level     string    `json:"level"` // info, warning, error
	Message   string    `json:"message"`
	Step      int       `json:"step"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatRequest represents a chat request from the user
type ChatRequest struct {
	Message string                 `json:"message"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// ChatResponse represents a streaming response
type ChatResponse struct {
	Type    string      `json:"type"` // plan, log, complete, error
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}
