// Package ansiext provides ANSI extension utilities.
package ansiext

// Escape represents an ANSI escape sequence.
type Escape string

// String returns the string representation of the escape.
func (e Escape) String() string {
	return string(e)
}

// StripStyles removes ANSI styles from a string.
func StripStyles(s string) string {
	// Simple implementation - remove ANSI escape sequences
	result := ""
	inEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1B {
			inEscape = true
		} else if inEscape && (c >= 'A' && c <= 'z') {
			// End of escape sequence
			inEscape = false
		} else if !inEscape {
			result += string(c)
		}
	}
	return result
}

// HasStyles checks if a string contains ANSI styles.
func HasStyles(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1B {
			return true
		}
	}
	return false
}
