package subagent

import (
	"context"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// Manager manages multiple subagent instances
type Manager struct {
	mu 				sync.RWMutex
	subagents 		map[string]*Subagent
	toolRegistry	map[string]types.Tool
	llmConfig 		LLMConfig
}

// New Manager creates a new Subagent Mananger
func NewManager(toolRegistry map[string]types.Tool, llmConfig LLMConfig) *Manager {
	return &Manager {
		subagents: 	  make(map[string]*Subagent),
		toolRegistry: toolRegistry,
		llmConfig: 	  llmConfig,
	}
}


// Spawn creates and starts new Subagent
func(m *Manager) Spawn(config SubagentConfig) *Subagent {
	// Apply Defaults
	if config.ID == "" {
		config.ID = generateID()
	}
	if config.Timeout == 0 {
		config.Timeout = 2 * time.Minute
	}
	if config.MaxSteps == 0 {
		config.MaxSteps = 20
	}

	// Create Subagent
	sub := New(config, m.toolRegistry, m.llmConfig)

	// Track it
	m.mu.Lock()
	m.subagents[sub.ID] = sub
	m.mu.Unlock()

	return sub
}

// SpawnandWait creates a subagent, executes it and then waits for result
func (m *Manager) SpawnAndWait(ctx context.Context, config SubagentConfig) *Result {
	sub := m.Spawn(config)
	return sub.Execute(ctx)
}


// SpawnAsync: Creates a subagent and executes it in background
func (m *Manager) SpawnAsync(ctx context.Context, config SubagentConfig) *Subagent{
	sub := m.Spawn(config)
	go sub.Execute(ctx)
	return sub
}

// Get: Retrieves a subagent by ID
func (m*Manager) Get(id string) *Subagent{
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subagents[id]
}

// List: Returns all subagent IDs
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.subagents))
	for id := range m.subagents {
		ids = append(ids, id)
	}
	return ids
}

// WaitForAll: Block Until All Subagents Complete or Timeout
func (m *Manager) WaitForAll(ctx context.Context, timeout time.Duration) map[string]*Result {
	results := make(map[string]*Result)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll until all complete or timeout
	for {
		select{
		case <-ctx.Done():
			return results
		default:
			m.mu.RLock()
			allDone := true
			for id, sub := range m.subagents {
				if sub.Status() == StatusRunning || sub.Status() == StatusPending {
					allDone = false
				} else if sub.Result() != nil && results[id] == nil {
					results[id] = sub.Result()
				}
			}
			m.mu.RUnlock()

			if allDone {
				return results
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Cleanup: Removes Completed Subagents older than duration
func (m *Manager) Cleanup(olderThan time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	for id, sub := range m.subagents {
		if sub.Status() != StatusRunning && sub.startTime.Before(cutoff){
			delete(m.subagents, id)
		}
	}
}