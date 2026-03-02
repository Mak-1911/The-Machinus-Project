package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

//SessionStore defines the interface for session storage
type SessionStore interface {
	SaveSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	CleanupExpiredSessions(ctx context.Context) (int, error)
}

//SessionManager manages session lifecycles
type SessionManager struct {
	store SessionStore
}

//NewSessionManager creates a new session manager
func NewSessionManager(store SessionStore) *SessionManager {
	return &SessionManager{
		store: store,
	}
}

//CreateSession creates a new session
func (sm *SessionManager) CreateSession(ctx context.Context) (*Session, error){
	session := &Session{
		ID: 		uuid.New().String(),
		StartedAt:	time.Now(),
		LastActive: time.Now(),
		Messages:	[]ConversationMessage{},
		Status: 	"active",
		Metadata:	make(map[string]string),
	}

	if err := sm.store.SaveSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	return session, nil
}

//GetOrCreateDefaultSession gets or creates the default session
func(sm *SessionManager) GetOrCreateDefaultSession(ctx context.Context, sessionID string) (*Session, error) {
	// If sessionID provided, try to load it
	if sessionID != "" {
		session, err := sm.store.GetSession(ctx, sessionID)
		if err == nil && session != nil{
			// Check if expired
			if session.IsExpired() {
				// Creates a new Session
				return sm.CreateSession(ctx)
			}
			// Update last active
			session.LastActive = time.Now()
			sm.store.SaveSession(ctx, session)
			return session, nil
		}
	}

	// Create a new Session
	return sm.CreateSession(ctx)
}

//AddMessage adds a message to a session
func(sm *SessionManager) AddMessage(ctx context.Context, sessionID, role, content, toolID string) error {
	session, err := sm.store.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	session.AddMessage(role, content, toolID)
	return sm.store.SaveSession(ctx, session)
}

//GetSession retrieves a session by ID
func(sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return sm.store.GetSession(ctx, sessionID)
}

//ListSessions lists all sessions
func(sm *SessionManager) ListSessions(ctx context.Context) ([]Session, error) {
	return sm.store.ListSessions(ctx)
}

//CloseSession closes a session
func(sm *SessionManager) CloseSession(ctx context.Context, sessionID string) error {
	session, err := sm.store.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	session.Status = "closed"
	session.LastActive = time.Now()
	return sm.store.SaveSession(ctx, session)
}

//ClearSession clears all messages from a session
func(sm *SessionManager) ClearSession(ctx context.Context, sessionID string) error {
	session, err := sm.store.GetSession(ctx, sessionID)
	if err!= nil {
		return fmt.Errorf("Session not found: %w", err)
	}

	session.ClearMessages()
	return sm.store.SaveSession(ctx, session)
}

//DeleteSession deletes a session
func(sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	return sm.store.DeleteSession(ctx, sessionID)
}

// CleanupExpired removes all expired sessions
func(sm *SessionManager) CleanupExpired(ctx context.Context) (int, error) {
	return sm.store.CleanupExpiredSessions(ctx)
}

