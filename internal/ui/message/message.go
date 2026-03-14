// Package message provides message types for the UI.
// This is an adapter package that bridges UI needs with internal types.
package message

import (
	"strings"
	"time"
)

// Role is the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Shorthand constants for convenience
const (
	User      = RoleUser
	Assistant = RoleAssistant
	System    = RoleSystem
	Tool      = RoleTool
)

// Status represents the status of a message or tool call.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusComplete Status = "complete"
	StatusError    Status = "error"
	StatusCanceled Status = "canceled"
)

// FinishReason represents the reason a message finished.
type FinishReason string

const (
	FinishReasonEndTurn   FinishReason = "end_turn"
	FinishReasonMaxTokens FinishReason = "max_tokens"
	FinishReasonStopped   FinishReason = "stopped"
	FinishReasonToolUse   FinishReason = "tool_use"
	FinishReasonError     FinishReason = "error"
	FinishReasonCanceled  FinishReason = "canceled"
)

// Message represents a message in the conversation.
type Message struct {
	id               string
	sessionID        string
	role             Role
	contentText      string
	binaryContent    []byte
	attachments      []Attachment
	createdAt        time.Time
	CreatedAt        int64
	toolCalls        []ToolCall
	metadata         map[string]string
	thinkingContent  ThinkingContent
	finishReason     FinishReason
	finishPart       *FinishPart
	isSummaryMessage bool
	isThinking       bool
	provider         string
	model            string
	toolResults      []ToolResult
}

// NewMessage creates a new message.
func NewMessage(id string, role Role, content string) *Message {
	return &Message{
		id:          id,
		role:        role,
		contentText: content,
		createdAt:   time.Now(),
		toolCalls:   make([]ToolCall, 0),
		metadata:    make(map[string]string),
	}
}

// ID returns the message ID.
func (m Message) ID() string {
	return m.id
}

// GetID returns the message ID (alias for compatibility).
func (m *Message) GetID() string {
	return m.id
}

// SetID sets the message ID.
func (m *Message) SetID(id string) {
	m.id = id
}

// Role returns the message role.
func (m *Message) Role() Role {
	return m.role
}

// Content returns the message content.
func (m *Message) Content() Content {
	return Content{
		Text: m.contentText,
	}
}

// BinaryContent returns the binary content.
func (m *Message) BinaryContent() []byte {
	return m.binaryContent
}

// Attachments returns the attachments.
func (m *Message) Attachments() []Attachment {
	return m.attachments
}

// ReasoningContent returns the reasoning content.
func (m *Message) ReasoningContent() ThinkingContent {
	return m.thinkingContent
}

// FinishReason returns the finish reason.
func (m *Message) FinishReason() FinishReason {
	return m.finishReason
}

// FinishPart returns the finish part.
func (m *Message) FinishPart() *FinishPart {
	return m.finishPart
}

// IsFinished checks if the message is finished.
func (m *Message) IsFinished() bool {
	return m.finishReason != "" || m.isSummaryMessage
}

// IsThinking checks if the message is currently thinking.
func (m *Message) IsThinking() bool {
	return m.isThinking
}

// IsSummaryMessage returns whether this is a summary message.
func (m *Message) IsSummaryMessage() bool {
	return m.isSummaryMessage
}

// ThinkingDuration returns the duration of thinking.
func (m *Message) ThinkingDuration() time.Duration {
	return 0 // Placeholder
}

// ToolCalls returns the tool calls.
func (m *Message) ToolCalls() []ToolCall {
	return m.toolCalls
}

// AddToolCall adds a tool call to the message.
func (m *Message) AddToolCall(id, name, input string) *ToolCall {
	tc := ToolCall{
		ID:    id,
		Name:  name,
		Input: input,
	}
	m.toolCalls = append(m.toolCalls, tc)
	return &m.toolCalls[len(m.toolCalls)-1]
}

// SetContent sets the message content.
func (m *Message) SetContent(content string) {
	m.contentText = content
}

// SetThinkingContent sets the thinking content.
func (m *Message) SetThinkingContent(content ThinkingContent) {
	m.thinkingContent = content
}

// SetFinishReason sets the finish reason.
func (m *Message) SetFinishReason(reason FinishReason) {
	m.finishReason = reason
}

// SetIsThinking sets the thinking state.
func (m *Message) SetIsThinking(thinking bool) {
	m.isThinking = thinking
}

// SetIsSummaryMessage sets the summary message flag.
func (m *Message) SetIsSummaryMessage(summary bool) {
	m.isSummaryMessage = summary
}

// Provider returns the message provider.
func (m *Message) Provider() string {
	return m.provider
}

// Model returns the message model.
func (m *Message) Model() string {
	return m.model
}

// SetProvider sets the message provider.
func (m *Message) SetProvider(provider string) {
	m.provider = provider
}

// SetModel sets the message model.
func (m *Message) SetModel(model string) {
	m.model = model
}

// ToolResults returns the tool results.
func (m *Message) ToolResults() []ToolResult {
	return m.toolResults
}

// AddToolResult adds a tool result to the message.
func (m *Message) AddToolResult(result ToolResult) {
	m.toolResults = append(m.toolResults, result)
}

// SessionID returns the session ID.
func (m Message) SessionID() string {
	return m.sessionID
}

// SetSessionID sets the session ID.
func (m *Message) SetSessionID(sessionID string) {
	m.sessionID = sessionID
}

// ThinkingContent represents thinking/reasoning content.
type ThinkingContent struct {
	Thinking string `json:"thinking"`
}

// Content represents text content.
type Content struct {
	Text string
}

// FinishPart represents a finish part with error details.
type FinishPart struct {
	Reason  FinishReason `json:"reason"`
	Message string       `json:"message"`
	Details string       `json:"details"`
	Time    int64        `json:"time"`
}

// ToolCall represents a call to a tool.
type ToolCall struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Input    string      `json:"input"`
	Response *ToolResult `json:"response,omitempty"`
	Finished bool        `json:"finished,omitempty"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
	Metadata   string `json:"metadata,omitempty"`
	Data       string `json:"data,omitempty"`
	MIMEType   string `json:"mime_type,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolResultStatus is the status of a tool result.
type ToolResultStatus string

const (
	ToolResultStatusPending  ToolResultStatus = "pending"
	ToolResultStatusComplete ToolResultStatus = "complete"
	ToolResultStatusError    ToolResultStatus = "error"
	ToolResultStatusCanceled ToolResultStatus = "canceled"
)

// NewToolResult creates a new tool result.
func NewToolResult(content string, isError bool) *ToolResult {
	return &ToolResult{
		Content: content,
		IsError: isError,
	}
}

// Attachment represents a file attachment.
type Attachment struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"file_path"`
	FileName    string    `json:"file_name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	MimeType    string    `json:"mime_type"`
	Bytes       []byte    `json:"bytes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetContent returns the attachment content bytes.
func (a *Attachment) GetContent() []byte {
	return a.Bytes
}

// IsImage checks if the attachment is an image.
func (a *Attachment) IsImage() bool {
	switch a.ContentType {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// ContainsTextAttachment checks if the given attachments contain text attachments.
func ContainsTextAttachment(attachments []Attachment) bool {
	for _, a := range attachments {
		if strings.HasPrefix(a.ContentType, "text/") {
			return true
		}
	}
	return false
}
