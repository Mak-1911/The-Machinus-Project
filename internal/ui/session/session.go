// Package session provides session types for the UI.
package session

import "time"

// Session represents a user session.
type Session struct {
	ID        string         `json:"id"`
	dir       string         `json:"dir"`
	title     string         `json:"title"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Metadata  SessionMetadata `json:"metadata"`
}

// Title returns the session title.
func (s *Session) Title() string {
	if s.title != "" {
		return s.title
	}
	return s.Metadata.Title
}

// SetTitle sets the session title.
func (s *Session) SetTitle(title string) {
	s.title = title
}

// CompletionTokens returns completion tokens.
func (s *Session) CompletionTokens() int {
	return 0
}

// PromptTokens returns prompt tokens.
func (s *Session) PromptTokens() int {
	return 0
}

// Todos returns todos.
func (s *Session) GetTodos() []Todo {
	return nil
}

// HasIncompleteTodos returns true if there are incomplete todos.
func (s *Session) HasIncompleteTodos() bool {
	return false
}

// HasIncompleteTodos is a standalone function.
func HasIncompleteTodos(todos []Todo) bool {
	return false
}

// SessionMetadata contains session metadata.
type SessionMetadata struct {
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Model        string    `json:"model"`
	MessageCount int       `json:"message_count"`
}

// SessionStatus is the status of a session.
type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "active"
	SessionStatusInactive SessionStatus = "inactive"
	SessionStatusArchived SessionStatus = "archived"
)

// Manager manages sessions.
type Manager struct {
	current *Session
	sessions map[string]*Session
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// Current returns the current session.
func (m *Manager) Current() *Session {
	return m.current
}

// SetCurrent sets the current session.
func (m *Manager) SetCurrent(s *Session) {
	m.current = s
}

// Add adds a session.
func (m *Manager) Add(s *Session) {
	m.sessions[s.ID] = s
}

// Get gets a session by ID.
func (m *Manager) Get(id string) (*Session, bool) {
	s, ok := m.sessions[id]
	return s, ok
}

// Delete deletes a session.
func (m *Manager) Delete(id string) {
	delete(m.sessions, id)
}

// List returns all sessions.
func (m *Manager) List() []*Session {
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// NewSession creates a new session.
func NewSession(id, dir string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		dir:       dir,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: SessionMetadata{
			Title: "Untitled Session",
		},
	}
}

// Todo represents a todo item.
type Todo struct {
	ID         string    `json:"id"`
	Message    string    `json:"message"`
	Content    string    `json:"content"`
	Status     TodoStatus `json:"status"`
	ActiveForm string    `json:"active_form"`
	CreatedAt  time.Time `json:"created_at"`
}

// TodoStatus represents the status of a todo item.
type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusComplete   TodoStatus = "complete"
	TodoStatusCanceled   TodoStatus = "canceled"
	TodoStatusCompleted  TodoStatus = "completed"
)
