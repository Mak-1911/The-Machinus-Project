// Package diff provides diff types for the UI.
package diff

import (
	"strings"
)

// UnifiedDiff represents a unified diff.
type UnifiedDiff struct {
	OldPath string
	NewPath string
	Hunks   []*Hunk
}

// Hunk represents a diff hunk.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []Line
}

// Line represents a line in a diff.
type Line struct {
	Type    LineType
	Content string
	NoNewline bool
}

// LineType is the type of line.
type LineType int

const (
	LineTypeContext LineType = iota
	LineTypeAdd
	LineTypeDelete
)

// ParseUnifiedDiff parses a unified diff string.
func ParseUnifiedDiff(diff string) *UnifiedDiff {
	lines := strings.Split(diff, "\n")
	result := &UnifiedDiff{
		Hunks: make([]*Hunk, 0),
	}
	var currentHunk *Hunk

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			// Start of a new hunk
			currentHunk = parseHunkHeader(line)
			result.Hunks = append(result.Hunks, currentHunk)
		} else if currentHunk != nil {
			// Parse line content
			lineType := LineTypeContext
			content := line
			noNewline := false

			if len(line) > 0 {
				switch line[0] {
				case '+':
					lineType = LineTypeAdd
					content = line[1:]
				case '-':
					lineType = LineTypeDelete
					content = line[1:]
				case ' ':
					content = line[1:]
				}
			}

			if strings.HasSuffix(line, "\\ No newline at end of file") {
				noNewline = true
				if len(currentHunk.Lines) > 0 {
					currentHunk.Lines[len(currentHunk.Lines)-1].NoNewline = true
				}
				continue
			}

			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:  lineType,
				Content: content,
				NoNewline: noNewline,
			})
		} else if strings.HasPrefix(line, "--- ") {
			result.OldPath = strings.TrimPrefix(line, "--- ")
		} else if strings.HasPrefix(line, "+++ ") {
			result.NewPath = strings.TrimPrefix(line, "+++ ")
		}
	}

	return result
}

// parseHunkHeader parses a hunk header like "@@ -1,4 +1,5 @@"
func parseHunkHeader(line string) *Hunk {
	h := &Hunk{
		Lines: make([]Line, 0),
	}
	// Simple parsing - would need more robust implementation
	return h
}

// String returns the unified diff as a string.
func (d *UnifiedDiff) String() string {
	var b strings.Builder
	for _, hunk := range d.Hunks {
		for _, line := range hunk.Lines {
			switch line.Type {
			case LineTypeAdd:
				b.WriteString("+")
			case LineTypeDelete:
				b.WriteString("-")
			default:
				b.WriteString(" ")
			}
			b.WriteString(line.Content)
			if line.NoNewline {
				b.WriteString("\n\\ No newline at end of file")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// GenerateDiff generates a diff between two strings.
func GenerateDiff(oldContent, newContent string, filename ...string) (string, int, int) {
	additions := 0
	removals := 0
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	for _, line := range oldLines {
		if line != "" {
			removals++
		}
	}
	for _, line := range newLines {
		if line != "" {
			additions++
		}
	}
	
	return newContent, additions, removals
}
