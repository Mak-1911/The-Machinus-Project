// Package util provides utility functions for UI message handling.
package util

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
	"mvdan.cc/sh/v3/shell"
)

type Cursor interface {
	Cursor() *tea.Cursor
}

func CmdHandler(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

func ReportError(err error) tea.Cmd {
	slog.Error("Error reported", "error", err)
	return CmdHandler(NewErrorMsg(err))
}

type InfoType int

const (
	InfoTypeInfo InfoType = iota
	InfoTypeSuccess
	InfoTypeWarn
	InfoTypeError
	InfoTypeUpdate
)

func NewInfoMsg(info string) InfoMsg {
	return InfoMsg{
		Type: InfoTypeInfo,
		Msg:  info,
	}
}

func NewWarnMsg(warn string) InfoMsg {
	return InfoMsg{
		Type: InfoTypeWarn,
		Msg:  warn,
	}
}

func NewErrorMsg(err error) InfoMsg {
	return InfoMsg{
		Type: InfoTypeError,
		Msg:  err.Error(),
	}
}

func ReportInfo(info string) tea.Cmd {
	return CmdHandler(NewInfoMsg(info))
}

func ReportWarn(warn string) tea.Cmd {
	return CmdHandler(NewWarnMsg(warn))
}

type (
	InfoMsg struct {
		Type InfoType
		Msg  string
		TTL  time.Duration
	}
	ClearStatusMsg struct{}
)

// IsEmpty checks if the [InfoMsg] is empty.
func (m InfoMsg) IsEmpty() bool {
	var zero InfoMsg
	return m == zero
}

// ExecShell parses a shell command string and executes it with exec.Command.
// Uses shell.Fields for proper handling of shell syntax like quotes and
// arguments while preserving TTY handling for terminal editors.
func ExecShell(ctx context.Context, cmdStr string, callback tea.ExecCallback) tea.Cmd {
	fields, err := shell.Fields(cmdStr, nil)
	if err != nil {
		return ReportError(err)
	}
	if len(fields) == 0 {
		return ReportError(errors.New("empty command"))
	}

	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	return tea.ExecProcess(cmd, callback)
}

// ============================================================================
// Additional Utility Functions
// ============================================================================

// FormatDuration formats a duration in a human-readable way.
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return formatInt(int(d.Milliseconds())) + "ms"
	}
	if d < time.Minute {
		return d.Truncate(time.Millisecond).String()
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return formatInt(mins) + "m" + formatInt(secs) + "s"
		}
		return formatInt(mins) + "m"
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins > 0 {
		return formatInt(hours) + "h" + formatInt(mins) + "m"
	}
	return formatInt(hours) + "h"
}

// FormatTime formats a time in a human-readable way.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return formatInt(mins) + "m ago"
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return formatInt(hours) + "h ago"
	}
	days := int(diff.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return formatInt(days) + "d ago"
}

// FormatBytes formats a byte count in a human-readable way.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return formatInt(int(b)) + " B"
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatInt(int(b/div)) + " " + []string{"B", "KB", "MB", "GB", "TB"}[exp+1]
}

func formatInt(n int) string {
	if n < 0 {
		return "0"
	}
	if n < 10 {
		return "0123456789"[n : n+1]
	}
	if n < 100 {
		return "0123456789"[n/10:n/10+1] + "0123456789"[n%10:n%10+1]
	}
	// Simple fallback
	s := ""
	for n > 0 {
		s = "0123456789"[n%10:n%10+1] + s
		n /= 10
	}
	return s
}

// Clamp clamps a value between min and max.
func Clamp(min, max, value int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Min returns the minimum of two values.
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two values.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Abs returns the absolute value of an integer.
func Abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
