// Package store provides filesystem-based session persistence.
package store

import "time"

// SessionMetadata represents the metadata stored in meta.json.
type SessionMetadata struct {
	ID           string    `json:"id"`
	WorkDir      string    `json:"work_dir"`
	Title        string    `json:"title"`
	CreatedAt    int64     `json:"created_at"`
	UpdatedAt    int64     `json:"updated_at"`
	LastActive   int64     `json:"last_active"`
	Model        string    `json:"model"`
	MessageCount int       `json:"message_count"`
	TokenCount   int       `json:"token_count,omitempty"`
}

// NewSessionMetadata creates new session metadata.
func NewSessionMetadata(id, workDir, title, model string) SessionMetadata {
	now := time.Now().Unix()
	return SessionMetadata{
		ID:           id,
		WorkDir:      workDir,
		Title:        title,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActive:   now,
		Model:        model,
		MessageCount: 0,
	}
}

// UpdateLastActive updates the last active timestamp.
func (m *SessionMetadata) UpdateLastActive() {
	m.LastActive = time.Now().Unix()
	m.UpdatedAt = time.Now().Unix()
}

// IncrementMessageCount increments the message count.
func (m *SessionMetadata) IncrementMessageCount() {
	m.MessageCount++
	m.UpdatedAt = time.Now().Unix()
}

// SessionSummary represents a summary for the session.
type SessionSummary struct {
	GeneratedAt int64  `json:"generated_at"`
	Content     string `json:"content"`
}
