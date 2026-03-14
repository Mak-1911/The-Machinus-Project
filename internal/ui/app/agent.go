// Package app provides the agent coordinator for LLM integration.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/config"
	uiconfig "github.com/machinus/cloud-agent/internal/ui/config"
	"github.com/machinus/cloud-agent/internal/ui/message"
)

// MessageHistoryProvider provides conversation history for a session.
type MessageHistoryProvider interface {
	GetHistoryForSession(ctx context.Context, sessionID string) ([]*message.Message, error)
}

// llmCoordinator manages LLM interactions.
type llmCoordinator struct {
	mu            sync.RWMutex
	client        *LLMClient
	busy          bool
	cancelFuncs   map[string]context.CancelFunc
	pendingCount  map[string]int
	config        *config.Config
	uiConfig      *uiconfig.UIConfig
	toolExecutor  *ToolExecutor
	workDir       string
	lastToolCalls []ExecutedToolCall
	progressCB    ProgressCallback
	historyProvider MessageHistoryProvider
	debug         bool
}

// LLMClient represents the API client for LLM.
type LLMClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// ToolCall represents a parsed tool call from the LLM response.
type ToolCall struct {
	ID     string
	Name   string
	Args   map[string]any
	Raw    string
	Result string
}

// NewAgentCoordinator creates a new agent coordinator.
func NewAgentCoordinator(cfg *config.Config, uiCfg *uiconfig.UIConfig) AgentCoordinator {
	workDir := "." // Default to current directory

	return &llmCoordinator{
		client: &LLMClient{
			BaseURL: cfg.LLMBaseURL,
			APIKey:  cfg.LLMAPIKey,
			Model:   cfg.LLMModel,
			HTTPClient: &http.Client{
				Timeout: 0,
			},
		},
		cancelFuncs:  make(map[string]context.CancelFunc),
		pendingCount: make(map[string]int),
		config:       cfg,
		uiConfig:     uiCfg,
		toolExecutor: NewToolExecutor(workDir),
		workDir:      workDir,
	}
}

// GetToolExecutor returns the tool executor.
func (a *llmCoordinator) GetToolExecutor() *ToolExecutor {
	return a.toolExecutor
}

// SetWorkingDir sets the working directory for tool execution.
func (a *llmCoordinator) SetWorkingDir(dir string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.workDir = dir
	a.toolExecutor = NewToolExecutor(dir)
}

// SetProgressCallback sets a callback for progress updates during execution.
func (a *llmCoordinator) SetProgressCallback(cb ProgressCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.progressCB = cb
}

// SetHistoryProvider sets the message history provider.
func (a *llmCoordinator) SetHistoryProvider(provider MessageHistoryProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.historyProvider = provider
}

// SetDebug sets debug mode for verbose logging.
func (a *llmCoordinator) SetDebug(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.debug = enabled
}

// sendProgress sends a progress event if callback is set.
func (a *llmCoordinator) sendProgress(event ProgressEvent) {
	a.mu.RLock()
	cb := a.progressCB
	a.mu.RUnlock()
	if cb != nil {
		cb(event)
	}
}

// getModelConfig returns the current model config.
func (a *llmCoordinator) getModelConfig() (baseURL, apiKey, model string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Check UI config first (for onboarding selected model)
	modelCfg, ok := a.uiConfig.Models()[uiconfig.SelectedModelTypeLarge]
	if ok && modelCfg.Model != "" {
		model = modelCfg.Model
		// Get base URL from provider config
		if modelCfg.Provider != "" {
			_, providerOk := a.uiConfig.Providers().Get(modelCfg.Provider)
			if providerOk {
				switch strings.ToLower(modelCfg.Provider) {
				case "anthropic":
					baseURL = "https://api.anthropic.com/v1"
				case "openai":
					baseURL = "https://api.openai.com/v1"
				case "openrouter":
					baseURL = "https://openrouter.ai/api/v1"
				case "zai":
					baseURL = "https://api.z.ai/api/coding/paas/v4"
				default:
					baseURL = a.config.LLMBaseURL
				}
			}
		}
		apiKey = a.config.LLMAPIKey
		return
	}

	baseURL = a.config.LLMBaseURL
	apiKey = a.config.LLMAPIKey
	model = a.config.LLMModel
	return
}

// Run executes a prompt and returns the response.
func (a *llmCoordinator) Run(ctx context.Context, sessionID, content string, attachments ...message.Attachment) (string, error) {
	a.mu.Lock()
	a.busy = true
	a.lastToolCalls = nil // Clear previous tool calls
	if a.pendingCount[sessionID] == 0 {
		a.pendingCount[sessionID] = 1
	} else {
		a.pendingCount[sessionID]++
	}
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.pendingCount[sessionID]--
		if a.pendingCount[sessionID] <= 0 {
			delete(a.pendingCount, sessionID)
		}
		a.busy = false
		a.mu.Unlock()
	}()

	baseURL, apiKey, model := a.getModelConfig()

	if apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}
	if model == "" {
		return "", fmt.Errorf("model not configured")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelFuncs[sessionID] = cancel
	a.mu.Unlock()

	messages := a.buildMessages(ctx, sessionID, content, attachments)

	// Initial LLM call
	response, err := a.callLLM(ctx, baseURL, apiKey, model, messages)
	if err != nil {
		return "", err
	}

	// Check if response contains tool calls
	maxIterations := 10 // Prevent infinite loops
	for iteration := 0; iteration < maxIterations; iteration++ {
		toolCalls := a.parseToolCalls(response)
		if len(toolCalls) == 0 {
			// No more tool calls, return the final response
			return response, nil
		}

		// Send tool_start events for all tools immediately
		for _, tc := range toolCalls {
			argsJSON, _ := json.Marshal(tc.Args)
			a.sendProgress(ProgressEvent{
				Type:     "tool_start",
				ToolID:   tc.ID,
				ToolName: tc.Name,
				ToolArgs: string(argsJSON),
			})
		}

		// Small delay to ensure UI processes the start events
		time.Sleep(50 * time.Millisecond)

		// Execute each tool call and collect results
		var toolResults []string
		for _, tc := range toolCalls {
			// Execute the tool
			result, err := a.executeToolCall(ctx, tc)
			isError := false
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
				isError = true
			}

			// Store the tool call for later retrieval
			argsJSON, _ := json.Marshal(tc.Args)
			a.mu.Lock()
			a.lastToolCalls = append(a.lastToolCalls, ExecutedToolCall{
				Name:   tc.Name,
				Args:   string(argsJSON),
				Result: result,
			})
			a.mu.Unlock()

			// Send progress update for completed tool
			a.sendProgress(ProgressEvent{
				Type:     "tool_complete",
				ToolID:   tc.ID,
				ToolName: tc.Name,
				ToolArgs: string(argsJSON),
				Result:   result,
				IsError:  isError,
			})

			toolResults = append(toolResults, fmt.Sprintf("Tool: %s\nResult: %s", tc.Name, result))
		}

		// Add assistant message and tool results to conversation
		messages = append(messages, Message{Role: "assistant", Content: response})
		messages = append(messages, Message{Role: "user", Content: "Tool results:\n" + strings.Join(toolResults, "\n\n")})

		// Get next response from LLM
		response, err = a.callLLM(ctx, baseURL, apiKey, model, messages)
		if err != nil {
			return "", err
		}
	}

	return response, nil
}

// GetLastToolCalls returns the tool calls from the last Run execution.
func (a *llmCoordinator) GetLastToolCalls() []ExecutedToolCall {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastToolCalls
}

// parseToolCalls parses tool calls from the LLM response.
// Supports format: tool:name\n{...} or ```tool:name\n{...}``` or ```bash\ncommand```
func (a *llmCoordinator) parseToolCalls(response string) []ToolCall {
	var calls []ToolCall

	// Debug: log the raw response to help diagnose parsing issues
	lines := strings.Split(response, "\n")
	debugLog := fmt.Sprintf("[%s] Split into %d lines:\n", time.Now().Format("15:04:05.000"), len(lines))
	for i, line := range lines {
		debugLog += fmt.Sprintf("  Line %d: %q\n", i, line)
	}
	_ = os.WriteFile("parser_debug.log", []byte(debugLog), 0644)

	// Pattern 0: tool:name\n{json_args} or tool:name {json_args} (simple format without backticks)
	// Also handles tool:name appearing anywhere in text (not just line start)
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Check if line starts with tool:
		toolIdx := strings.Index(line, "tool:")
		if toolIdx < 0 {
			continue // No tool: prefix in this line
		}

		// Extract from tool: onwards (handles tool: appearing mid-line)
		line = line[toolIdx:]

		afterTool := strings.TrimPrefix(line, "tool:")
		afterTool = strings.TrimSpace(afterTool)

		// Split tool name from potential inline args (e.g., "list_files {}" -> name="list_files", args="{}")
		var toolName string
		var inlineArgs string
		if spaceIdx := strings.Index(afterTool, " "); spaceIdx > 0 {
			toolName = afterTool[:spaceIdx]
			inlineArgs = strings.TrimSpace(afterTool[spaceIdx:])
		} else if spaceIdx := strings.Index(afterTool, "{"); spaceIdx > 0 {
			toolName = afterTool[:spaceIdx]
			inlineArgs = strings.TrimSpace(afterTool[spaceIdx:])
		} else {
			toolName = afterTool
		}

		// Collect following lines as JSON args
		var argsLines []string
		if inlineArgs != "" {
			argsLines = append(argsLines, inlineArgs)
		}
		emptyLineCount := 0
		for j := i + 1; j < len(lines); j++ {
			nextLine := lines[j]
			trimmedLine := strings.TrimSpace(nextLine)

			// Skip empty lines right after tool: directive
			if trimmedLine == "" && len(argsLines) == 0 {
				continue
			}

			// Stop at double newline or another tool: directive
			if trimmedLine == "" {
				emptyLineCount++
				if emptyLineCount >= 2 {
					break
				}
				continue
			}
			emptyLineCount = 0

			if strings.HasPrefix(trimmedLine, "tool:") {
				break
			}
			argsLines = append(argsLines, nextLine)
		}

		argsJSON := strings.TrimSpace(strings.Join(argsLines, "\n"))

		// If no args found, use empty object
		if argsJSON == "" {
			argsJSON = "{}"
		}

		// Try to extract just the JSON part (find first { and last })
		startIdx := strings.Index(argsJSON, "{")
		endIdx := strings.LastIndex(argsJSON, "}")
		if startIdx >= 0 && endIdx > startIdx {
			argsJSON = argsJSON[startIdx : endIdx+1]
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			// Try parsing as plain text for simple commands
			args = map[string]any{"cmd": argsJSON}
		}

		calls = append(calls, ToolCall{
			ID:   fmt.Sprintf("call_%d", len(calls)),
			Name: toolName,
			Args: args,
			Raw:  line + "\n" + strings.Join(argsLines, "\n"),
		})
	}

	// Pattern 1: ```tool:tool_name\n{json_args}``` (code block format)
	toolPattern := regexp.MustCompile("```tool:(\\w+)\\s*\\n([\\s\\S]*?)\\n```")
	matches := toolPattern.FindAllStringSubmatch(response, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			toolName := match[1]
			argsJSON := strings.TrimSpace(match[2])
			var args map[string]any
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				args = map[string]any{"cmd": argsJSON}
			}
			calls = append(calls, ToolCall{
				ID:   fmt.Sprintf("call_%d", len(calls)),
				Name: toolName,
				Args: args,
				Raw:  match[0],
			})
		}
	}

	// Pattern 2: ```bash\ncommand``` -> convert to bash tool
	bashPattern := regexp.MustCompile("```bash\\s*\\n([\\s\\S]*?)\\n```")
	bashMatches := bashPattern.FindAllStringSubmatch(response, -1)
	for _, match := range bashMatches {
		if len(match) >= 2 {
			command := strings.TrimSpace(match[1])
			if command != "" {
				calls = append(calls, ToolCall{
					ID:   fmt.Sprintf("bash_%d", len(calls)),
					Name: "bash",
					Args: map[string]any{"cmd": command},
					Raw:  match[0],
				})
			}
		}
	}

	// Pattern 3: ```shell\ncommand``` -> convert to shell tool
	shellPattern := regexp.MustCompile("```shell\\s*\\n([\\s\\S]*?)\\n```")
	shellMatches := shellPattern.FindAllStringSubmatch(response, -1)
	for _, match := range shellMatches {
		if len(match) >= 2 {
			command := strings.TrimSpace(match[1])
			if command != "" {
				calls = append(calls, ToolCall{
					ID:   fmt.Sprintf("shell_%d", len(calls)),
					Name: "shell",
					Args: map[string]any{"cmd": command},
					Raw:  match[0],
				})
			}
		}
	}

	return calls
}

// executeToolCall executes a single tool call.
func (a *llmCoordinator) executeToolCall(ctx context.Context, tc ToolCall) (string, error) {
	if a.toolExecutor == nil {
		return "", fmt.Errorf("tool executor not available")
	}

	result, err := a.toolExecutor.Execute(ctx, tc.Name, tc.Args)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return result.Error, nil
	}

	return result.Output, nil
}

// buildMessages constructs the message list from conversation history.
func (a *llmCoordinator) buildMessages(ctx context.Context, sessionID, content string, attachments []message.Attachment) []Message {
	messages := []Message{
		{Role: "system", Content: a.getSystemPrompt()},
	}

	// Add conversation history if available
	if a.historyProvider != nil {
		if history, err := a.historyProvider.GetHistoryForSession(ctx, sessionID); err == nil && len(history) > 0 {
			// Convert UI messages to LLM messages
			for _, msg := range history {
				// Skip tool messages and system messages
				if msg.Role() == message.RoleTool || msg.Role() == message.RoleSystem {
					continue
				}
				// Skip thinking/summary messages
				if msg.IsThinking() || msg.IsSummaryMessage() {
					continue
				}
				role := string(msg.Role())
				content := msg.Content().Text
				if content != "" {
					messages = append(messages, Message{
						Role:    role,
						Content: content,
					})
				}
			}
		}
	}

	// Add current user message
	messages = append(messages, Message{Role: "user", Content: content})

	return messages
}

// getSystemPrompt returns the system prompt.
func (a *llmCoordinator) getSystemPrompt() string {
	basePrompt := `You are a helpful AI assistant. You can use tools to complete tasks, but you should also answer questions conversationally when appropriate.

Tool Usage Guidelines:
- ONLY use tools when the user explicitly asks for file operations, searching, or code execution
- For conversational questions like "what did we do?", "explain this", etc., just answer directly without tools
- When you DO use tools, call them with proper JSON arguments
- After using tools, provide a clear response based on the results

Respond naturally and helpfully. Don't over-use tools for simple conversation.`

	// Add tool instructions if tools are available
	if a.toolExecutor != nil && len(a.toolExecutor.GetTools()) > 0 {
		basePrompt += "\n\n" + a.toolExecutor.GetToolsPrompt()
	}

	return basePrompt
}

// callLLM makes the actual API call to the LLM.
func (a *llmCoordinator) callLLM(ctx context.Context, baseURL, apiKey, model string, messages []Message) (string, error) {
	// Build URL - just append /chat/completions like the existing planner does
	url := baseURL + "/chat/completions"

	req := ChatRequest{
		Model:    model,
		Messages: messages,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqJSON)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// IsSessionBusy checks if session is busy.
func (a *llmCoordinator) IsSessionBusy(id string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pendingCount[id] > 0
}

// Model returns the current model name.
func (a *llmCoordinator) Model() string {
	_, _, model := a.getModelConfig()
	return model
}

// QueuedPromptsList returns queued prompts.
func (a *llmCoordinator) QueuedPromptsList() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var list []string
	for id, count := range a.pendingCount {
		if count > 0 {
			list = append(list, fmt.Sprintf("%s: %d pending", id, count))
		}
	}
	return list
}

// QueuedPrompts returns queued prompts count for a session.
func (a *llmCoordinator) QueuedPrompts(sessionID string) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pendingCount[sessionID]
}

// Summarize summarizes a session (placeholder).
func (a *llmCoordinator) Summarize(ctx context.Context, sessionID string) error {
	return nil
}

// IsBusy checks if the agent is busy.
func (a *llmCoordinator) IsBusy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.busy
}

// Cancel cancels the agent coordinator for a session.
func (a *llmCoordinator) Cancel(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cancel, ok := a.cancelFuncs[sessionID]; ok {
		cancel()
		delete(a.cancelFuncs, sessionID)
	}
	return nil
}

// ClearQueue clears the agent coordinator queue.
func (a *llmCoordinator) ClearQueue(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pendingCount[sessionID] = 0
	return nil
}
