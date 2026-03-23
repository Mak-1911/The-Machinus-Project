// Package store provides filesystem-based session persistence.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/machinus/cloud-agent/internal/agent"
)

// SessionSaver saves messages to a session.
type SessionSaver struct {
	store   *FilesystemStore
	workDir string
}

// NewSessionSaver creates a new session saver.
func NewSessionSaver(store *FilesystemStore, workDir string) *SessionSaver {
	return &SessionSaver{
		store:   store,
		workDir: workDir,
	}
}

// SetWorkingDir sets the working directory for the session saver.
func (s *SessionSaver) SetWorkingDir(dir string) {
	s.workDir = dir
}

// SaveMessage saves a message to the session.
func (s *SessionSaver) SaveMessage(ctx context.Context, sessionID, role, content string) error {
	// Get the existing session
	session, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		// Session might not exist yet, create it
		session = &agent.Session{
			ID:         sessionID,
			StartedAt:  time.Now(),
			LastActive: time.Now(),
			Messages:   []agent.ConversationMessage{},
			Status:     "active",
			Metadata: map[string]string{
				"work_dir": s.workDir,
				"title":    "New Session",
			},
		}
	}

	// Add the message
	msg := agent.ConversationMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().Unix(),
	}
	session.Messages = append(session.Messages, msg)
	session.LastActive = time.Now()

	// Save the updated session
	return s.store.SaveSession(ctx, session)
}

// CreateNewSession creates a new session with an optional title.
func (s *SessionSaver) CreateNewSession(ctx context.Context, title string) (*agent.Session, error) {
	sessionID := uuid.New().String()
	session := &agent.Session{
		ID:         sessionID,
		StartedAt:  time.Now(),
		LastActive: time.Now(),
		Messages:   []agent.ConversationMessage{},
		Status:     "active",
		Metadata: map[string]string{
			"work_dir": s.workDir,
			"title":    title,
		},
	}

	if err := s.store.SaveSession(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (s *SessionSaver) GetSession(ctx context.Context, sessionID string) (*agent.Session, error) {
	return s.store.GetSession(ctx, sessionID)
}

// ListSessionsByWorkDir lists all sessions for the current working directory.
func (s *SessionSaver) ListSessionsByWorkDir(ctx context.Context) ([]agent.Session, error) {
	return s.store.ListSessionsByWorkDir(ctx, s.workDir)
}

// GetLatestSession returns the most recent session for the current working directory.
func (s *SessionSaver) GetLatestSession(ctx context.Context) (*agent.Session, error) {
	sessions, err := s.store.ListSessionsByWorkDir(ctx, s.workDir)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found for working directory: %s", s.workDir)
	}
	return &sessions[0], nil // List returns sorted by last_active descending
}

// SessionFile represents the session file on disk.
type SessionFile struct {
	Path string
	Meta SessionMetadata
}

// GetSessionFiles returns all session files for a working directory.
func (s *SessionSaver) GetSessionFiles(ctx context.Context) ([]SessionFile, error) {
	sessions, err := s.store.ListSessionsByWorkDir(ctx, s.workDir)
	if err != nil {
		return nil, err
	}

	var files []SessionFile
	for _, session := range sessions {
		workDir := session.Metadata["work_dir"]
		sessionDir := SessionDir(s.store.rootDir, workDir, session.ID)

		// Read metadata
		metaPath := filepath.Join(sessionDir, "meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var meta SessionMetadata
		if err := json.Unmarshal(metaData, &meta); err != nil {
			continue
		}

		files = append(files, SessionFile{
			Path: sessionDir,
			Meta: meta,
		})
	}

	return files, nil
}
