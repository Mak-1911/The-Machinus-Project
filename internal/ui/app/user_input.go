// Package app provides user input integration for the AskUserInput tool.
package app

import (
	"fmt"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/tools"
)

// Global user input manager singleton
var globalUserInputManager = &UserInputManager{
	pendingReqs: make(map[string]*pendingUserInputDialogRequest),
}

// GetGlobalUserInputManager returns the global user input manager.
func GetGlobalUserInputManager() *UserInputManager {
	return globalUserInputManager
}

// UserInputManager manages pending user input requests and responses.
type UserInputManager struct {
	mu          sync.RWMutex
	pendingReqs map[string]*pendingUserInputDialogRequest
	progressCB  ProgressCallback
}

// pendingUserInputDialogRequest represents a pending user input dialog with channels.
type pendingUserInputDialogRequest struct {
	RequestID   string
	Message     string
	Placeholder string
	Default     string
	Options     []string
	ResponseCh  chan string
	CancelCh    chan struct{}
}

// SetProgressCallback sets the callback for notifying UI about input requests.
func (m *UserInputManager) SetProgressCallback(cb ProgressCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressCB = cb
}

// notifyUI sends a progress event to the UI to show the input dialog.
func (m *UserInputManager) notifyUI(req *pendingUserInputDialogRequest) {
	m.mu.RLock()
	cb := m.progressCB
	m.mu.RUnlock()

	if cb != nil {
		// Copy options for the public request
		var options []string
		if len(req.Options) > 0 {
			options = make([]string, len(req.Options))
			copy(options, req.Options)
		}

		cb(ProgressEvent{
			Type: "ask_user_input",
			UserInputRequest: &UserInputDialogRequest{
				RequestID:   req.RequestID,
				Message:     req.Message,
				Placeholder: req.Placeholder,
				Default:     req.Default,
				Options:     options,
			},
		})
	}
}

// RegisterToolCallback sets up the callback on the AskUserInput tool.
// This should be called during app initialization.
func (m *UserInputManager) RegisterToolCallback(askTool *tools.AskUserInputTool) {
	askTool.SetCallback(m.handleInputRequest)
}

// handleInputRequest is called by the AskUserInput tool when it needs user input.
func (m *UserInputManager) handleInputRequest(req tools.UserInputRequest) (string, error) {
	// Create channels for this request
	responseCh := make(chan string, 1)
	cancelCh := make(chan struct{}, 1)

	// Create internal request
	dialogReq := &pendingUserInputDialogRequest{
		RequestID:   req.ID,
		Message:     req.Message,
		Placeholder: req.Placeholder,
		Default:     req.Default,
		Options:     req.Options,
		ResponseCh:  responseCh,
		CancelCh:    cancelCh,
	}

	// Store the request
	m.mu.Lock()
	m.pendingReqs[req.ID] = dialogReq
	m.mu.Unlock()

	// Notify the UI to show the dialog
	m.notifyUI(dialogReq)

	// Wait for response or timeout
	select {
	case response := <-responseCh:
		m.mu.Lock()
		delete(m.pendingReqs, req.ID)
		m.mu.Unlock()
		return response, nil

	case <-cancelCh:
		m.mu.Lock()
		delete(m.pendingReqs, req.ID)
		m.mu.Unlock()
		return "", fmt.Errorf("user cancelled the input")

	case <-time.After(5 * time.Minute):
		m.mu.Lock()
		delete(m.pendingReqs, req.ID)
		m.mu.Unlock()
		return "", fmt.Errorf("input request timed out")
	}
}

// SubmitResponse submits a user's response to a pending request.
// This is called by the TUI when the user submits the dialog.
func (m *UserInputManager) SubmitResponse(requestID, response string) error {
	m.mu.RLock()
	req, ok := m.pendingReqs[requestID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no pending request with ID %s", requestID)
	}

	select {
	case req.ResponseCh <- response:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("failed to send response")
	}
}

// CancelRequest cancels a pending request.
// This is called by the TUI when the user cancels the dialog.
func (m *UserInputManager) CancelRequest(requestID string) error {
	m.mu.RLock()
	req, ok := m.pendingReqs[requestID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no pending request with ID %s", requestID)
	}

	select {
	case req.CancelCh <- struct{}{}:
		return nil
	default:
		return fmt.Errorf("failed to cancel request")
	}
}

// GetPendingRequest returns info about a pending request for the UI.
func (m *UserInputManager) GetPendingRequest(requestID string) *UserInputDialogRequest {
	m.mu.RLock()
	req, ok := m.pendingReqs[requestID]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	// Copy options for the public request
	var options []string
	if len(req.Options) > 0 {
		options = make([]string, len(req.Options))
		copy(options, req.Options)
	}

	return &UserInputDialogRequest{
		RequestID:   req.RequestID,
		Message:     req.Message,
		Placeholder: req.Placeholder,
		Default:     req.Default,
		Options:     options,
	}
}

// SetupAskUserInputTool sets up the AskUserInput tool with the global manager.
// Call this during app initialization after creating the tool executor.
func SetupAskUserInputTool(askTool *tools.AskUserInputTool) {
	globalUserInputManager.RegisterToolCallback(askTool)
}
