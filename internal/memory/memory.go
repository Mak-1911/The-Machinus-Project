package memory

import (
	"context"
	"fmt"
	"time"
)

// Memory represents a stored memory
type Memory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Summary   string    `json:"summary"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager handles memory storage and retrieval
type Manager struct {
	store       Store
	enabled     bool
	maxMemories int
}

// Store defines the interface for memory persistence
type Store interface {
	SaveMemory(ctx context.Context, memory *Memory) error
	GetMemoriesByUserID(ctx context.Context, userID string, limit int) ([]Memory, error)
	SearchSimilar(ctx context.Context, userID string, query string, limit int) ([]Memory, error)
}

// NewManager creates a new memory manager
func NewManager(store Store, enabled bool, maxMemories int) *Manager {
	return &Manager{
		store:       store,
		enabled:     enabled,
		maxMemories: maxMemories,
	}
}

// Retrieve fetches relevant memories for a query
func (m *Manager) Retrieve(ctx context.Context, userID, query string) ([]Memory, error) {
	if !m.enabled {
		return nil, nil
	}

	// Search for similar memories using text search
	memories, err := m.store.SearchSimilar(ctx, userID, query, m.maxMemories)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}

	return memories, nil
}

// Store saves a new memory
func (m *Manager) Store(ctx context.Context, memory *Memory) error {
	if !m.enabled {
		return nil
	}

	// Save to store
	if err := m.store.SaveMemory(ctx, memory); err != nil {
		return fmt.Errorf("failed to save memory: %w", err)
	}

	return nil
}

// SummarizeResult creates a memory from a task result
func (m *Manager) SummarizeResult(ctx context.Context, userID, taskID, description, result string) (*Memory, error) {
	if !m.enabled {
		return nil, nil
	}

	// For MVP, create a simple memory
	// In production, you'd use LLM to generate better summaries
	memory := &Memory{
		UserID:  userID,
		Summary: fmt.Sprintf("Task %s: %s", taskID, description),
		Content: result,
		Tags:    []string{"task-result"},
	}

	return memory, nil
}
