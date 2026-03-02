package agent

import (
	"time"
)

// Session represents a conversation session
type Session struct {
	ID 			string 				  `json:"id"`
	StartedAt	time.Time 			  `json:"started_at"`
	LastActive	time.Time 			  `json:"last_active"`
	Messages 	[]ConversationMessage `json:"messages"`
	Status 		string 				  `json:"status"`
	Metadata 	map[string]string     `json:"metadata,omitempty"`
}

//ConversationMessage represents a message in the conversation
type ConversationMessage struct {
	Role  		string  `json:"role"`  //"user", "assistant", "tool"
	Content 	string 	`json:"content"`
	ToolID		string 	`json:"tool_id,omitempty"`
	Timestamp 	int64 	`json:"timestamp"`
}

//IsExpired checks if the session is expired (24 hours of inactivity)
func (s *Session) IsExpired() bool {
	return time.Since(s.LastActive) > 24*time.Hour
}

//IsActive checks if the session is active
func (s *Session) IsActive() bool {
	return s.Status == "active" && !s.IsExpired()
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(role, content, toolID string) {
	msg := ConversationMessage{
		Role: 		role,
		Content:	content,
		ToolID: 	toolID,
		Timestamp:	time.Now().Unix(),
	}
	s.Messages = append(s.Messages, msg)
	s.LastActive = time.Now()
}

// Get RecentMessages return the last N messages
func (s *Session) GetRecentMessages(n int) []ConversationMessage {
	if len(s.Messages) <= n {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-n:]
}

// ClearMessages clears all messages from the session
func (s *Session) ClearMessages() {
	s.Messages = []ConversationMessage{}
	s.LastActive = time.Now()
}