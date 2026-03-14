// Package app provides application context types for the UI.
package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/config"
	uiconfig "github.com/machinus/cloud-agent/internal/ui/config"
	"github.com/machinus/cloud-agent/internal/ui/message"
	"github.com/machinus/cloud-agent/internal/ui/session"
)

// ProgressEvent represents a progress update during agent execution.
type ProgressEvent struct {
	Type      string // "tool_start", "tool_complete", "thinking", "error"
	ToolID    string
	ToolName  string
	ToolArgs  string
	Result    string
	IsError   bool
}

// ProgressCallback is called when there's a progress update.
type ProgressCallback func(ProgressEvent)

// AgentCoordinator defines the interface for LLM agent coordination.
type AgentCoordinator interface {
	IsSessionBusy(id string) bool
	Model() string
	QueuedPromptsList() []string
	QueuedPrompts(sessionID string) int
	Summarize(ctx context.Context, sessionID string) error
	Run(ctx context.Context, sessionID, content string, attachments ...message.Attachment) (string, error)
	IsBusy() bool
	Cancel(sessionID string) error
	ClearQueue(sessionID string) error
	GetLastToolCalls() []ExecutedToolCall
	SetProgressCallback(cb ProgressCallback)
}

// ExecutedToolCall represents a tool that was executed.
type ExecutedToolCall struct {
	Name   string
	Args   string
	Result string
}

// App represents the application context.
type App struct {
	ctx      context.Context
	cancel   context.CancelFunc
	config   *config.Config
	uiConfig *uiconfig.UIConfig
	mu       sync.RWMutex
	quitCh   chan struct{}
	Sessions        *sessionMgr
	AgentCoordinator AgentCoordinator
	LSPManager      *LSPManager
	Messages        *Messages
	FileTracker     *FileTracker
	History         *History
	Permissions     *permissionMgr
}

// New creates a new app context.
func New(cfg *config.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())
	uiCfg := uiconfig.NewUIConfig(cfg)
	return &App{
		ctx:         ctx,
		cancel:      cancel,
		config:      cfg,
		uiConfig:    uiCfg,
		quitCh:      make(chan struct{}),
		Sessions:    &sessionMgr{},
		AgentCoordinator: NewAgentCoordinator(cfg, uiCfg),
		LSPManager:  &LSPManager{},
		Messages:    &Messages{},
		FileTracker: &FileTracker{},
		History:     &History{},
		Permissions: &permissionMgr{},
	}
}

// Context returns the app context.
func (a *App) Context() context.Context {
	return a.ctx
}

// Config returns the app config.
func (a *App) Config() *config.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// UIConfig returns the UI config wrapper with providers.
func (a *App) UIConfig() *uiconfig.UIConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.uiConfig
}

// SetConfig sets the app config.
func (a *App) SetConfig(cfg *config.Config) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg
}

// UpdateAgentModel updates the agent model from config.
func (a *App) UpdateAgentModel(ctx context.Context) error {
	// The AgentCoordinator reads from config on each call
	// This method can be used to trigger any necessary updates
	return nil
}

// Quit signals the app to quit.
func (a *App) Quit() {
	close(a.quitCh)
	a.cancel()
}

// QuitChan returns the quit channel.
func (a *App) QuitChan() <-chan struct{} {
	return a.quitCh
}

// Done returns a channel that's closed when the app is done.
func (a *App) Done() <-chan struct{} {
	return a.ctx.Done()
}

// sessionMgr manages sessions.
type sessionMgr struct {
	mu        sync.RWMutex
	sessions  map[string]*session.Session
}

// List returns sessions.
func (s *sessionMgr) List(ctx context.Context) ([]session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []session.Session
	for _, sess := range s.sessions {
		result = append(result, *sess)
	}
	return result, nil
}

// Delete deletes a session.
func (s *sessionMgr) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

// Save saves a session.
func (s *sessionMgr) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = make(map[string]*session.Session)
	}
	s.sessions[sess.ID] = &sess
	return sess, nil
}

// Get returns a session by ID.
func (s *sessionMgr) Get(id string) (*session.Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sessions == nil {
		return nil, false
	}
	sess, ok := s.sessions[id]
	return sess, ok
}

// Create creates a new session.
func (s *sessionMgr) Create(ctx context.Context, title string) (*session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = make(map[string]*session.Session)
	}
	now := time.Now()
	sess := &session.Session{
		ID:        generateSessionID(),
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: session.SessionMetadata{
			Title: title,
		},
	}
	s.sessions[sess.ID] = sess
	return sess, nil
}

// SetSession sets a session directly (for external session sources).
func (s *sessionMgr) SetSession(sess *session.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = make(map[string]*session.Session)
	}
	s.sessions[sess.ID] = sess
}

// CreateAgentToolSessionID creates a session ID for an agent tool.
func (s *sessionMgr) CreateAgentToolSessionID(messageID, toolCallID string) string { return "" }

// ParseAgentToolSessionID parses an agent tool session ID.
func (s *sessionMgr) ParseAgentToolSessionID(id string) (string, string, bool) { return "", "", false }

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	return fmt.Sprintf("sess-%d", time.Now().UnixNano())
}

// LSPClientInfo represents LSP client information.
type LSPClientInfo struct {
	Name    string
	Version string
}

// LSPEvent represents an LSP event.
type LSPEvent struct {
	Type string
	Data any
}

// LSPManager represents LSP manager.
type LSPManager struct {
	clients interface{}
}

// Clients returns LSP clients.
func (l *LSPManager) Clients() interface{} { return l.clients }

// SetClients sets LSP clients.
func (l *LSPManager) SetClients(clients interface{}) { l.clients = clients }

// Start starts LSP manager for a path.
func (l *LSPManager) Start(ctx context.Context, path string) error {
	return nil // Placeholder
}

// StartWithPath starts LSP manager for a path.
func (l *LSPManager) StartWithPath(ctx context.Context, path string) error {
	return l.Start(ctx, path)
}

// StopAll stops all LSP managers.
func (l *LSPManager) StopAll(ctx context.Context) error { return nil }

// Messages represents message store.
type Messages struct {
	mu       sync.RWMutex
	messages map[string][]*message.Message // sessionID -> messages
}

// Add adds a message to the store.
func (m *Messages) Add(msg *message.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.messages == nil {
		m.messages = make(map[string][]*message.Message)
	}
	sessionID := msg.SessionID()
	m.messages[sessionID] = append(m.messages[sessionID], msg)
}

// Get returns a message by ID.
func (m *Messages) Get(id string) *message.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, msgs := range m.messages {
		for _, msg := range msgs {
			if msg.ID() == id {
				return msg
			}
		}
	}
	return nil
}

// List returns all messages for a session.
func (m *Messages) List(ctx context.Context, sessionID string) ([]*message.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.messages == nil {
		return nil, nil
	}
	msgs, ok := m.messages[sessionID]
	if !ok {
		return nil, nil
	}
	// Return a copy to avoid race conditions
	result := make([]*message.Message, len(msgs))
	copy(result, msgs)
	return result, nil
}

// ListUserMessages returns user messages.
func (m *Messages) ListUserMessages(ctx context.Context, sessionID string) ([]*message.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.messages == nil {
		return nil, nil
	}
	var result []*message.Message
	for _, msg := range m.messages[sessionID] {
		if msg.Role() == message.User {
			result = append(result, msg)
		}
	}
	return result, nil
}

// ListAllUserMessages returns all user messages.
func (m *Messages) ListAllUserMessages(ctx context.Context) ([]*message.Message, error) { return nil, nil }

// History returns history.
func (m *Messages) History() *History { return &History{} }

// GetLSPStates returns LSP states.
func (a *App) GetLSPStates() map[string]*LSPClientInfo { return nil }

// FileTracker returns file tracker.
type FileTracker struct{}

// Get returns file by path.
func (f *FileTracker) Get(path string) interface{} { return nil }

// GetAll returns all files.
func (f *FileTracker) GetAll() []interface{} { return nil }

// ListReadFiles returns read files.
func (f *FileTracker) ListReadFiles() []string { return nil }

// LastReadTime returns the last read time for a file.
func (f *FileTracker) LastReadTime(ctx context.Context, sessionID, path string) time.Time { return time.Time{} }

// RecordRead records a file read.
func (f *FileTracker) RecordRead(ctx context.Context, sessionID, path string) error { return nil }

// History returns history manager.
type History struct{}

// Get returns history entry by path.
func (h *History) Get(path string) interface{} { return nil }

// GetAll returns all history entries.
func (h *History) GetAll() []interface{} { return nil }

// permissionMgr is a placeholder for permission manager.
type permissionMgr struct{}

// SkipRequests returns whether to skip permission requests.
func (p *permissionMgr) SkipRequests() bool { return false }

// SetSkipRequests sets whether to skip permission requests.
func (p *permissionMgr) SetSkipRequests(skip bool) {}

// Grant grants a permission.
func (p *permissionMgr) Grant(perm string) error { return nil }

// GrantPersistent grants a persistent permission.
func (p *permissionMgr) GrantPersistent(perm string) error { return nil }

// Deny denies a permission.
func (p *permissionMgr) Deny(perm string) error { return nil }

// GetDefaultSmallModel returns the default small model for a provider.
func (a *App) GetDefaultSmallModel(provider string) string { return "" }

// InitCoderAgent initializes the coder agent.
func (a *App) InitCoderAgent(ctx context.Context) error { return nil }
