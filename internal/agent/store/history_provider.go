// Package store provides filesystem-based session persistence.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinus/cloud-agent/internal/ui/message"
)

// FileHistoryProvider implements MessageHistoryProvider using the filesystem.
type FileHistoryProvider struct {
	store *FilesystemStore
}

// NewFileHistoryProvider creates a new file-based history provider.
func NewFileHistoryProvider(store *FilesystemStore) *FileHistoryProvider {
	return &FileHistoryProvider{
		store: store,
	}
}

// GetHistoryForSession retrieves conversation history for a session.
func (p *FileHistoryProvider) GetHistoryForSession(ctx context.Context, sessionID string) ([]*message.Message, error) {
	// Find the session directory
	sessionDir, err := p.store.findSessionDir(sessionID)
	if err != nil {
		return nil, err
	}

	// Load messages from messages.jsonl
	messagesPath := filepath.Join(sessionDir, "messages.jsonl")
	messages, err := p.loadMessages(messagesPath, sessionID)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// loadMessages loads messages from messages.jsonl and converts them to message.Message.
func (p *FileHistoryProvider) loadMessages(path, sessionID string) ([]*message.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*message.Message{}, nil // No messages yet
		}
		return nil, fmt.Errorf("failed to read messages: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var messages []*message.Message

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse as ConversationMessage first
		var convMsg ConversationMessage
		if err := json.Unmarshal([]byte(line), &convMsg); err != nil {
			continue // Skip malformed lines
		}

		// Convert to message.Message using NewMessage
		msg := message.NewMessage(generateID(), message.Role(convMsg.Role), convMsg.Content)
		msg.SetSessionID(sessionID)
		msg.CreatedAt = convMsg.Timestamp

		messages = append(messages, msg)
	}

	return messages, nil
}

// ConversationMessage represents a message in the conversation (stored format).
type ConversationMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolID    string `json:"tool_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// generateID generates a simple ID for messages.
func generateID() string {
	return fmt.Sprintf("%d", parseTime(0).UnixNano())
}
