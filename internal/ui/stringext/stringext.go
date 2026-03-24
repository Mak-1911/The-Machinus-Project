// Package stringext provides string extensions for the UI.
package stringext

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Truncate truncates a string to a maximum length, adding "..." if truncated.
func Truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

// ElideMiddle truncates the middle of a string, keeping the beginning and end.
func ElideMiddle(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return "..."
	}
	runes := []rune(s)
	halfLen := (maxLen - 3) / 2
	return string(runes[:halfLen]) + "..." + string(runes[len(runes)-halfLen:])
}

// indent indents each line of a string by a given number of spaces.
func Indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// Dedent removes common leading whitespace from each line.
func Dedent(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return s
	}

	// Skip empty lines at start
	start := 0
	for start < len(lines) && len(strings.TrimSpace(lines[start])) == 0 {
		start++
	}
	if start >= len(lines) {
		return strings.Join(lines, "\n")
	}

	// Find minimum leading whitespace
	minIndent := -1
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if len(line) == 0 {
			continue
		}
		indent := 0
		for _, r := range line {
			if !unicode.IsSpace(r) {
				break
			}
			indent++
		}
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return s
	}

	// Remove minIndent from each non-empty line
	for i := start; i < len(lines); i++ {
		if len(lines[i]) >= minIndent {
			lines[i] = lines[i][minIndent:]
		}
	}

	return strings.Join(lines, "\n")
}

// SplitLines splits a string into lines, preserving empty lines.
func SplitLines(s string) []string {
	return strings.Split(s, "\n")
}

// JoinLines joins strings with newline separators.
func JoinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// NormalizeSpace normalizes whitespace in a string.
func NormalizeSpace(s string) string {
	// Replace all whitespace sequences with single spaces
	words := strings.Fields(s)
	return strings.Join(words, " ")
}

// EmptyTo returns a default value if the string is empty.
func EmptyTo(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
}

// Capitalize capitalizes the first letter of a string.
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// PreserveNewlines normalizes spaces but keeps line breaks.
// Use this for tool output that should maintain line structure.
func PreserveNewlines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// Trim leading/trailing whitespace from each line
		line = strings.TrimLeftFunc(line, unicode.IsSpace)
		line = strings.TrimRightFunc(line, unicode.IsSpace)

		// Collapse multiple spaces within the line
		var result strings.Builder
		inSpace := false
		for _, r := range line {
			if unicode.IsSpace(r) {
				if !inSpace {
					result.WriteRune(' ')
					inSpace = true
				}
			} else {
				result.WriteRune(r)
				inSpace = false
			}
		}
		lines[i] = result.String()
	}
	return strings.Join(lines, "\n")
}
