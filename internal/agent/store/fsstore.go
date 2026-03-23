// Package store provides filesystem-based session persistence.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/agent"
)

// FilesystemStore implements SessionStore using the filesystem.
type FilesystemStore struct {
	rootDir string
}

// NewFilesystemStore creates a new filesystem store.
func NewFilesystemStore(rootDir string) *FilesystemStore {
	return &FilesystemStore{
		rootDir: rootDir,
	}
}

// ensureDir creates a directory if it doesn't exist.
func (f *FilesystemStore) ensureDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// SaveSession saves a session to the filesystem.
func (f *FilesystemStore) SaveSession(ctx context.Context, session *agent.Session) error {
	workDir, ok := session.Metadata["work_dir"]
	if !ok {
		return fmt.Errorf("session missing work_dir metadata")
	}

	sessionDir := SessionDir(f.rootDir, workDir, session.ID)
	if err := f.ensureDir(sessionDir); err != nil {
		return err
	}

	// Save metadata
	metaPath := filepath.Join(sessionDir, "meta.json")
	if err := f.saveMetadata(metaPath, session); err != nil {
		return err
	}

	// Save messages
	messagesPath := filepath.Join(sessionDir, "messages.jsonl")
	if err := f.saveMessages(messagesPath, session); err != nil {
		return err
	}

	return nil
}

// saveMetadata saves session metadata to meta.json.
func (f *FilesystemStore) saveMetadata(path string, session *agent.Session) error {
	meta := SessionMetadata{
		ID:           session.ID,
		WorkDir:      session.Metadata["work_dir"],
		Title:        session.Metadata["title"],
		CreatedAt:    session.StartedAt.Unix(),
		UpdatedAt:    session.LastActive.Unix(),
		LastActive:   session.LastActive.Unix(),
		Model:        session.Metadata["model"],
		MessageCount: len(session.Messages),
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// saveMessages saves messages to messages.jsonl (one JSON per line).
func (f *FilesystemStore) saveMessages(path string, session *agent.Session) error {
	var lines []string
	for _, msg := range session.Messages {
		data, err := json.Marshal(msg)
		if err != nil {
			continue // Skip messages that can't be marshaled
		}
		lines = append(lines, string(data))
	}

	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write messages: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID.
func (f *FilesystemStore) GetSession(ctx context.Context, sessionID string) (*agent.Session, error) {
	// Need to search for the session directory since we don't have workDir
	sessionDir, err := f.findSessionDir(sessionID)
	if err != nil {
		return nil, err
	}

	// Load metadata
	metaPath := filepath.Join(sessionDir, "meta.json")
	meta, err := f.loadMetadata(metaPath)
	if err != nil {
		return nil, err
	}

	// Load messages
	messagesPath := filepath.Join(sessionDir, "messages.jsonl")
	messages, err := f.loadMessages(messagesPath)
	if err != nil {
		return nil, err
	}

	// Build session
	session := &agent.Session{
		ID:         meta.ID,
		StartedAt:  parseTime(meta.CreatedAt),
		LastActive: parseTime(meta.LastActive),
		Messages:   messages,
		Status:     "active",
		Metadata: map[string]string{
			"work_dir": meta.WorkDir,
			"title":    meta.Title,
			"model":    meta.Model,
		},
	}

	return session, nil
}

// findSessionDir searches for a session directory by session ID.
func (f *FilesystemStore) findSessionDir(sessionID string) (string, error) {
	// Look for session-<id> in any subdirectory
	var found string

	err := filepath.Walk(f.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), "session-"+sessionID) {
			found = path
			return filepath.SkipAll // Stop walking
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	return found, nil
}

// loadMetadata loads metadata from meta.json.
func (f *FilesystemStore) loadMetadata(path string) (*SessionMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta SessionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// loadMessages loads messages from messages.jsonl.
func (f *FilesystemStore) loadMessages(path string) ([]agent.ConversationMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []agent.ConversationMessage{}, nil // No messages yet
		}
		return nil, fmt.Errorf("failed to read messages: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var messages []agent.ConversationMessage

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg agent.ConversationMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue // Skip malformed lines
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// ListSessions lists all sessions.
func (f *FilesystemStore) ListSessions(ctx context.Context) ([]agent.Session, error) {
	var sessions []agent.Session

	// Walk through all directories in rootDir
	entries, err := os.ReadDir(f.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []agent.Session{}, nil // No sessions yet
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Look for session directories inside this working directory
		workDirPath := filepath.Join(f.rootDir, entry.Name())
		sessionDirs, err := os.ReadDir(workDirPath)
		if err != nil {
			continue
		}

		for _, sd := range sessionDirs {
			if !sd.IsDir() || !strings.HasPrefix(sd.Name(), "session-") {
				continue
			}

			// Load session metadata
			metaPath := filepath.Join(workDirPath, sd.Name(), "meta.json")
			meta, err := f.loadMetadata(metaPath)
			if err != nil {
				continue
			}

			session := agent.Session{
				ID:         meta.ID,
				StartedAt:  parseTime(meta.CreatedAt),
				LastActive: parseTime(meta.LastActive),
				Status:     "active",
				Metadata: map[string]string{
					"work_dir": meta.WorkDir,
					"title":    meta.Title,
					"model":    meta.Model,
				},
			}
			sessions = append(sessions, session)
		}
	}

	// Sort by last_active descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActive.After(sessions[j].LastActive)
	})

	return sessions, nil
}

// DeleteSession deletes a session by ID.
func (f *FilesystemStore) DeleteSession(ctx context.Context, sessionID string) error {
	sessionDir, err := f.findSessionDir(sessionID)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(sessionDir); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes expired sessions.
func (f *FilesystemStore) CleanupExpiredSessions(ctx context.Context) (int, error) {
	sessions, err := f.ListSessions(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, session := range sessions {
		if session.IsExpired() {
			if err := f.DeleteSession(ctx, session.ID); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// ListSessionsByWorkDir lists all sessions for a specific working directory.
func (f *FilesystemStore) ListSessionsByWorkDir(ctx context.Context, workDir string) ([]agent.Session, error) {
	sessions, err := f.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []agent.Session
	for _, session := range sessions {
		if session.Metadata["work_dir"] == workDir {
			filtered = append(filtered, session)
		}
	}

	return filtered, nil
}

// parseTime converts a Unix timestamp to time.Time.
func parseTime(sec int64) time.Time {
	if sec == 0 {
		return time.Now()
	}
	return time.Unix(sec, 0)
}
