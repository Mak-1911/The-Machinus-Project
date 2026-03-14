package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

)

// TUILogWriter captures log messages and sends them to the TUI via Bubbletea messages
type TUILogWriter struct {
	mu       sync.Mutex
	messageChan chan<- agentLogMsg
	taskID   string
	lastStep int
}

// NewTUILogWriter creates a new TUI log writer
func NewTUILogWriter(messageChan chan<- agentLogMsg) *TUILogWriter {
	return &TUILogWriter{
		messageChan: messageChan,
	}
}

// Write writes a log entry to the TUI message channel
func (w *TUILogWriter) Write(ctx context.Context, taskID, level, message string, step int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Filter noise - keep only important logs
	if message == "Processing request..." {
		return nil
	}
	if strings.Contains(message, "Iteration ") && strings.Contains(message, "Thinking...") {
		return nil
	}
	if strings.HasPrefix(message, "Final response:") {
		return nil
	}
	if strings.Contains(message, "Task completed") {
		return nil
	}
	if message == "Response:" {
		return nil
	}
	if strings.HasPrefix(message, "Plan:") {
		return nil
	}

	// Send to TUI via channel
	if w.messageChan != nil {
		// Truncate long messages
		maxLen := 500
		displayMessage := message
		if len(message) > maxLen {
			displayMessage = message[:maxLen] + "..."
		}

		w.messageChan <- agentLogMsg{
			taskID:  taskID,
			level:   level,
			message: displayMessage,
			step:    step,
		}
	}

	w.lastStep = step
	return nil
}

// SetTaskID sets the current task ID
func (w *TUILogWriter) SetTaskID(taskID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.taskID = taskID
}

// agentLogMsg represents a log message from the agent
type agentLogMsg struct {
	taskID  string
	level   string
	message string
	step    int
}

// sanitizeOutput removes non-printable characters from output
func sanitizeOutput(s string) string {
	// Check if content appears to be binary
	binaryCount := 0
	for _, r := range s {
		// Count non-printable characters (excluding whitespace)
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			binaryCount++
		}
		// Count extended ASCII characters that might be binary
		if r > 126 && r < 128 {
			binaryCount++
		}
	}

	// If more than 10% non-printable, treat as binary
	if len(s) > 0 && float64(binaryCount)/float64(len(s)) > 0.1 {
		return "(binary content filtered)"
	}

	// Otherwise, just remove non-printable characters
	var result strings.Builder
	for _, r := range s {
		if (r >= 32 && r <= 126) || r == '\n' || r == '\r' || r == '\t' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// FormatMessage formats a log message for display
func (m agentLogMsg) FormatMessage() string {
	timestamp := time.Now().Format("15:04:05")

	switch {
	case strings.Contains(m.message, "Executing:"):
		// Extract tool name
		parts := strings.SplitN(m.message, "Executing:", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("[%s] 🔧 Executing: %s", timestamp, strings.TrimSpace(parts[1]))
		}
		return fmt.Sprintf("[%s] 🔧 %s", timestamp, m.message)

	case strings.Contains(m.message, "Output:"):
		// Show output with indentation
		parts := strings.SplitN(m.message, "Output:", 2)
		if len(parts) == 2 {
			output := strings.TrimSpace(parts[1])
			// Sanitize output to remove binary content
			output = sanitizeOutput(output)
			// Truncate very long output
			if len(output) > 300 {
				output = output[:300] + "..."
			}
			return fmt.Sprintf("[%s]   Result: %s", timestamp, output)
		}
		return fmt.Sprintf("[%s]   %s", timestamp, m.message)

	case m.level == "error":
		return fmt.Sprintf("[%s] ❌ Error: %s", timestamp, m.message)

	case m.level == "warning":
		return fmt.Sprintf("[%s] ⚠️  %s", timestamp, m.message)

	default:
		// Info messages
		return fmt.Sprintf("[%s] ℹ️  %s", timestamp, m.message)
	}
}
