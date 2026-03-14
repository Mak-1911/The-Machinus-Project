// Package history provides history management for the UI.
package history

import (
	"sync"
)

// History manages command history.
type History struct {
	mu      sync.RWMutex
	items   []string
	maxSize int
	current int
}

// New creates a new history.
func New(maxSize int) *History {
	return &History{
		items:   make([]string, 0, maxSize),
		maxSize: maxSize,
		current: -1,
	}
}

// Add adds an item to history.
func (h *History) Add(item string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove any duplicates after current position
	h.items = append(h.items[:h.current+1], item)
	if len(h.items) > h.maxSize {
		h.items = h.items[len(h.items)-h.maxSize:]
	}
	h.current = len(h.items) - 1
}

// Previous returns the previous item in history.
func (h *History) Previous() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.current < 0 {
		return "", false
	}
	if h.current > 0 {
		h.current--
	}
	return h.items[h.current], true
}

// Next returns the next item in history.
func (h *History) Next() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.current >= len(h.items)-1 {
		return "", false
	}
	h.current++
	return h.items[h.current], true
}

// Reset resets the current position to the end.
func (h *History) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = len(h.items) - 1
}

// Items returns all items in history.
func (h *History) Items() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]string, len(h.items))
	copy(result, h.items)
	return result
}

// File represents a file in history.
type File struct {
	Path      string
	Content   string
	Version   int
	SessionID string
}
