package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ANSI color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
)

// ConsoleLogWriter streams logs to console in real-time
type ConsoleLogWriter struct {
	mu       sync.Mutex
	enabled  bool
	showAll  bool // Show all logs or just important ones
	lastStep int
}

// NewConsoleLogWriter creates a new console log writer
func NewConsoleLogWriter(enabled bool, showAll bool) *ConsoleLogWriter {
	return &ConsoleLogWriter{
		enabled: enabled,
		showAll: showAll,
	}
}

// Write writes a log entry to console
func (w *ConsoleLogWriter) Write(ctx context.Context, taskID, level, message string, step int) error {
	if !w.enabled {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Filter out noise
	if !w.showAll {
		// Skip iteration messages
		if strings.Contains(message, "Iteration ") && strings.Contains(message, "Thinking...") {
			return nil
		}
		// Skip processing messages
		if message == "Processing request..." {
			return nil
		}
		// Skip final response duplicate
		if strings.HasPrefix(message, "Final response:") {
			return nil
		}
		// Skip task completed messages
		if strings.Contains(message, "Task completed") {
			return nil
		}
		// Only show important logs
		if level == "info" && !strings.Contains(message, "Executing:") && !strings.Contains(message, "Output:") && !strings.Contains(message, "error") {
			return nil
		}
	}

	// Format the log entry
	prefix := ""

	switch level {
	case "error":
		prefix = fmt.Sprintf("  %s❌%s ", ColorRed, ColorReset)
	case "warning":
		prefix = fmt.Sprintf("  %s⚠️ %s ", ColorYellow, ColorReset)
	case "info":
		if strings.Contains(message, "Executing:") {
			prefix = fmt.Sprintf("  %s→%s ", ColorCyan, ColorReset)
		} else if strings.Contains(message, "Output:") {
			// Don't show output prefix, just indent
			prefix = "    "
		} else if strings.Contains(message, "error") {
			prefix = fmt.Sprintf("  %s⚠️ %s ", ColorYellow, ColorReset)
		}
	}

	// Truncate long messages
	maxLen := 200
	displayMessage := message
	if len(message) > maxLen {
		displayMessage = message[:maxLen] + "..."
	}

	if prefix != "" || level == "error" {
		fmt.Printf("%s%s\n", prefix, displayMessage)
	}

	w.lastStep = step

	return nil
}

// StreamingLogWriter combines storage and console streaming
type StreamingLogWriter struct {
	store    Store
	console   *ConsoleLogWriter
}

// NewStreamingLogWriter creates a new streaming log writer
func NewStreamingLogWriter(store Store, consoleEnabled, verbose bool) *StreamingLogWriter {
	return &StreamingLogWriter{
		store:  store,
		console: NewConsoleLogWriter(consoleEnabled, verbose),
	}
}

// Write writes a log entry to both storage and console
func (w *StreamingLogWriter) Write(ctx context.Context, taskID, level, message string, step int) error {
	// Save to storage
	log := &TaskLog{
		ID:        taskID + "_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		TaskID:    taskID,
		Level:     level,
		Message:   message,
		Step:      step,
		Timestamp: time.Now(),
	}
	w.store.SaveTaskLog(ctx, log)

	// Also stream to console
	if w.console != nil {
		w.console.Write(ctx, taskID, level, message, step)
	}

	return nil
}
