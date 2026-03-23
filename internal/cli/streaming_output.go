// Package cli provides console output utilities for the CLI.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/agent"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
	ColorBold   = "\033[1m"
)

// Spinner frames for loading animation
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StreamingOutput handles real-time output streaming to console
type StreamingOutput struct {
	mu              sync.Mutex
	out             io.Writer
	spinnerIdx      int
	isSpinning      bool
	stopSpinner     chan struct{}
	lastTool        string
	lastToolLine    string  // Store the tool line for overwriting
	showTimestamp   bool
}

// NewStreamingOutput creates a new streaming output handler
func NewStreamingOutput(showTimestamp bool) *StreamingOutput {
	return &StreamingOutput{
		out:           os.Stdout,
		stopSpinner:   make(chan struct{}),
		showTimestamp: showTimestamp,
	}
}

// StartSpinner begins the spinner animation
func (s *StreamingOutput) StartSpinner(message string) func() {
	s.mu.Lock()
	s.isSpinning = true
	s.mu.Unlock()

	// Start spinner in background
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				if !s.isSpinning {
					s.mu.Unlock()
					return
				}
				s.spinnerIdx = (s.spinnerIdx + 1) % len(spinnerFrames)
				timestamp := ""
				if s.showTimestamp {
					timestamp = time.Now().Format("15:04:05") + " "
				}
				fmt.Printf("\r%s%s%s %s%s", ColorCyan, spinnerFrames[s.spinnerIdx], ColorReset, timestamp, message)
				s.mu.Unlock()
			case <-s.stopSpinner:
				return
			case <-done:
				return
			}
		}
	}()

	// Return function to stop spinner
	return func() {
		s.mu.Lock()
		s.isSpinning = false
		close(done)
		// Clear the spinner line
		fmt.Printf("\r%s\r", strings.Repeat(" ", 100))
		s.mu.Unlock()
	}
}

// ToolExecuting shows a tool is starting execution (cyan bullet, no newline)
func (s *StreamingOutput) ToolExecuting(toolName string, args map[string]any) {
	s.mu.Lock()
	s.lastTool = toolName

	// Format args as command-like string: tool(arg1, arg2, ...)
	argsStr := ""
	if len(args) > 0 {
		parts := []string{}
		// Priority args for display
		priorityKeys := []string{"path", "pattern", "url", "query", "message", "cmd", "command", "glob"}
		for _, key := range priorityKeys {
			if v, ok := args[key]; ok {
				valStr := fmt.Sprintf("%v", v)
				// Truncate long values
				if len(valStr) > 40 {
					valStr = valStr[:37] + "..."
				}
				parts = append(parts, valStr)
			}
		}
		if len(parts) > 0 {
			argsStr = strings.Join(parts, ", ")
			if len(argsStr) > 50 {
				argsStr = argsStr[:47] + "..."
			}
		}
	}

	// Build tool line and store it for later overwriting
	s.lastToolLine = fmt.Sprintf("  %s•%s %s%s%s", ColorCyan, ColorReset, ColorBold, toolName, ColorReset)
	if argsStr != "" {
		s.lastToolLine += fmt.Sprintf("(%s%s%s)", ColorGray, argsStr, ColorReset)
	}
	// Print without newline - we'll overwrite it later
	fmt.Print(s.lastToolLine)
	s.mu.Unlock()
}

// ToolOutput shows tool output as it arrives
func (s *StreamingOutput) ToolOutput(output string) {
	if output == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Split into lines
	lines := strings.Split(output, "\n")
	// Filter empty lines
	var nonEmptyLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, strings.TrimSpace(line))
		}
	}

	if len(nonEmptyLines) == 0 {
		return
	}

	// Show first 2 lines - only first line gets ⌊ symbol
	maxLines := 2
	for i, line := range nonEmptyLines {
		if i >= maxLines {
			break
		}
		if len(line) > 100 {
			line = line[:97] + "..."
		}
		if i == 0 {
			// First line gets the ⌊ symbol
			fmt.Printf("\n    %s⌊%s %s", ColorGray, ColorReset, line)
		} else {
			// Subsequent lines just get indentation
			fmt.Printf("\n    %s%s", ColorGray, line)
		}
	}

	// Show +N more lines if there are more
	if len(nonEmptyLines) > maxLines {
		moreCount := len(nonEmptyLines) - maxLines
		fmt.Printf("\n    +%d more lines", moreCount)
	}
	fmt.Println() // Empty line after tool output
}

// plural returns "s" for plural or "" for singular
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ToolSuccess shows a tool completed successfully (overwrites line with green bullet + time)
func (s *StreamingOutput) ToolSuccess(toolName string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear the current line and print success with green bullet
	durStr := fmt.Sprintf("[%v]", duration.Round(time.Millisecond))
	successLine := fmt.Sprintf("  %s•%s %s%s%s [%s]%s\r", ColorGreen, ColorReset, ColorBold, toolName, ColorReset, durStr, ColorReset)
	// Clear any remaining characters from the original line
	clearLen := len(s.lastToolLine) + 20
	successLine += strings.Repeat(" ", clearLen-len(successLine))
	fmt.Print(successLine)
	fmt.Println() // Move to next line
}

// ToolError shows a tool failed (overwrites line with red bullet)
func (s *StreamingOutput) ToolError(toolName string, err string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Truncate error if too long
	if len(err) > 80 {
		err = err[:77] + "..."
	}

	// Clear the current line and print error with red bullet
	errorLine := fmt.Sprintf("  %s•%s %s%s%s: %s\r", ColorRed, ColorReset, ColorBold, toolName, ColorReset, ColorRed+err+ColorReset)
	clearLen := len(s.lastToolLine) + 20
	errorLine += strings.Repeat(" ", clearLen-len(errorLine))
	fmt.Print(errorLine)
	fmt.Println()
}

// Section shows a section header
func (s *StreamingOutput) Section(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("\n%s%s%s\n", ColorBold, title, ColorReset)
}

// Info shows an info message
func (s *StreamingOutput) Info(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("  %s•%s %s\n", ColorBlue, ColorReset, message)
}

// Warning shows a warning message
func (s *StreamingOutput) Warning(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("  %s⚠%s %s\n", ColorYellow, ColorReset, message)
}

// Error shows an error message
func (s *StreamingOutput) Error(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("  %s✗%s %s\n", ColorRed, ColorReset, message)
}

// Response shows the agent's final response
func (s *StreamingOutput) Response(response string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("\n%s\n", response) // No color - plain white for readability
}

// ProgressCallback creates a progress callback for the orchestrator
func (s *StreamingOutput) ProgressCallback() agent.ProgressCallback {
	return func(event agent.ProgressEvent) {
		switch event.Type {
		case "tool_start":
			s.ToolExecuting(event.ToolName, event.Args)
		case "tool_complete":
			if event.IsError {
				s.ToolError(event.ToolName, event.Result)
			} else {
				// Show output first
				if event.Result != "" {
					s.ToolOutput(event.Result)
				}
				s.ToolSuccess(event.ToolName, time.Duration(event.Duration)*time.Millisecond)
			}
		case "thinking":
			// Optional: show thinking indicator
		case "error":
			s.Error(event.Result)
		}
	}
}

// LogWriter creates a LogWriter for the orchestrator
func (s *StreamingOutput) LogWriter() agent.LogWriter {
	return &streamingLogWriter{
		output: s,
	}
}

// streamingLogWriter implements agent.LogWriter interface
type streamingLogWriter struct {
	output *StreamingOutput
}

func (w *streamingLogWriter) Write(ctx context.Context, taskID, level, message string, step int) error {
	switch level {
	case "error":
		w.output.Error(message)
	case "warning":
		w.output.Warning(message)
	case "info":
		if strings.Contains(message, "Executing:") {
			// Extract tool name from message
			parts := strings.SplitN(message, "Executing:", 2)
			if len(parts) == 2 {
				toolName := strings.TrimSpace(parts[1])
				w.output.ToolExecuting(toolName, nil)
			}
		}
	}
	return nil
}
