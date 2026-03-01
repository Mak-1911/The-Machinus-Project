package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/memory"
	"github.com/machinus/cloud-agent/internal/types"
)

// LLMClient represents the API client for GLM
type LLMClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// Planner generates execution plans using LLM
type Planner struct {
	client *LLMClient
	tools  map[string]types.Tool
}

// NewPlanner creates a new planner
func NewPlanner(baseURL, apiKey, model string, tools map[string]types.Tool) *Planner {
	return &Planner{
		client: &LLMClient{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
			HTTPClient: &http.Client{
				Timeout: 30 * time.Second,
			},
		},
		tools: tools,
	}
}

// PlanRequest represents the request to the LLM
type PlanRequest struct {
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
}

// Message represents a chat message (used for both requests and responses)
type Message struct {
	Role            string       `json:"role"`
	Content         string       `json:"content"`
	ToolCalls       []ToolCall   `json:"tool_calls,omitempty"`
	ReasoningContent string      `json:"reasoning_content,omitempty"`
}

// ToolDef represents a tool definition for the LLM (OpenAI format)
type ToolDef struct {
	Type     string           `json:"type"`
	Function ToolFunction     `json:"function"`
}

// ToolFunction represents the function part of a tool definition
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// PlanResponse represents the response from the LLM
type PlanResponse struct {
	Choices []Choice `json:"choices"`
}

// Choice represents a choice in the response
type Choice struct {
	Message    Message    `json:"message"`
}

// ToolCall represents a tool call in the response
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Index    int          `json:"index,omitempty"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ConversationMessage represents a message in the conversation history
type ConversationMessage struct {
	Role      string     `json:"role"`      // "user", "assistant", "tool"
	Content   string     `json:"content"`   // Message content
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolID    string     `json:"tool_id,omitempty"` // For tool responses
}

// Plan generates a plan from the user's message
// Returns (plan, message, error) - exactly one will be non-nil
func (p *Planner) Plan(ctx context.Context, message string, memories []memory.Memory) (*types.Plan, string, error) {
	// Build tool definitions in OpenAI format with parameters
	toolDefs := make([]ToolDef, 0, len(p.tools))
	for _, tool := range p.tools {
		params := p.buildToolParameters(tool)
		toolDefs = append(toolDefs, ToolDef{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  params,
			},
		})
	}

	// Build context
	systemPrompt := p.buildSystemPrompt(toolDefs)
	userPrompt := p.buildUserPrompt(message, memories)

	reqBody := PlanRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Tools: toolDefs,
	}

	// Call LLM
	response, err := p.client.Call(ctx, reqBody)
	if err != nil {
		return nil, "", fmt.Errorf("LLM call failed: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, "", fmt.Errorf("no choices in LLM response")
	}

	choice := response.Choices[0]

	// Parse the response - either tool calls or text message
	plan, textResponse, err := p.parseResponse(choice.Message.Content, choice.Message.ToolCalls)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	return plan, textResponse, nil
}

// Continue continues the conversation with message history
// Returns (toolCalls, textResponse, error) - exactly one will be non-nil
// This is used in the continuation loop to process tool results and get next actions
func (p *Planner) Continue(ctx context.Context, messages []ConversationMessage, memories []memory.Memory) ([]ToolCall, string, error) {
	// Build tool definitions in OpenAI format with parameters
	toolDefs := make([]ToolDef, 0, len(p.tools))
	for _, tool := range p.tools {
		params := p.buildToolParameters(tool)
		toolDefs = append(toolDefs, ToolDef{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  params,
			},
		})
	}

	// Build system prompt
	systemPrompt := p.buildSystemPrompt(toolDefs)

	// Build request messages - convert conversation to OpenAI format
	type OpenAIMessage struct {
		Role       string     `json:"role"`
		Content    string     `json:"content,omitempty"`
		ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
	}

	openAIMessages := []OpenAIMessage{{Role: "system", Content: systemPrompt}}

	for _, msg := range messages {
		if msg.Role == "tool" {
			// Tool result message - must include tool_call_id
			openAIMessages = append(openAIMessages, OpenAIMessage{
				Role:       "tool",
				Content:    msg.Content,
				ToolCallID: msg.ToolID,
			})
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Assistant message with tool calls
			openAIMessages = append(openAIMessages, OpenAIMessage{
				Role:      msg.Role,
				ToolCalls: msg.ToolCalls,
			})
		} else {
			// Regular user/assistant message
			openAIMessages = append(openAIMessages, OpenAIMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Add memories if available
	if len(memories) > 0 && len(messages) == 1 {
		// Only add memories to first user message
		var memInfo strings.Builder
		memInfo.WriteString("Relevant Context:\n")
		for i, mem := range memories {
			memInfo.WriteString(fmt.Sprintf("%d. %s\n", i+1, mem.Summary))
		}
		// Prepend to the first user message
		for i := range openAIMessages {
			if openAIMessages[i].Role == "user" {
				openAIMessages[i].Content = memInfo.String() + "\n\n" + openAIMessages[i].Content
				break
			}
		}
	}

	// Convert OpenAIMessages back to the Message format for the API call
	// We need to serialize them manually since the API expects a specific format
	reqMessages := make([]map[string]interface{}, 0, len(openAIMessages))
	for _, msg := range openAIMessages {
		msgMap := map[string]interface{}{
			"role": msg.Role,
		}
		if msg.Content != "" {
			msgMap["content"] = msg.Content
		}
		if len(msg.ToolCalls) > 0 {
			msgMap["tool_calls"] = msg.ToolCalls
		}
		if msg.ToolCallID != "" {
			msgMap["tool_call_id"] = msg.ToolCallID
		}
		reqMessages = append(reqMessages, msgMap)
	}

	// Call LLM with proper message format
	response, err := p.client.CallWithMessages(ctx, reqMessages, toolDefs)
	if err != nil {
		return nil, "", fmt.Errorf("LLM call failed: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, "", fmt.Errorf("no choices in LLM response")
	}

	choice := response.Choices[0]

	// Case 1: Tool calls returned in proper format
	if len(choice.Message.ToolCalls) > 0 {
		return choice.Message.ToolCalls, "", nil
	}

	// Case 2: Tool calls embedded in text content (some models do this)
	content := choice.Message.Content
	if content != "" && strings.Contains(content, "tool_calls") {
		// Try to parse tool_calls from JSON content
		var toolResponse struct {
			ToolCalls []struct {
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		}
		if err := json.Unmarshal([]byte(content), &toolResponse); err == nil && len(toolResponse.ToolCalls) > 0 {
			// Convert to our ToolCall format
			toolCalls := make([]ToolCall, len(toolResponse.ToolCalls))
			for i, tc := range toolResponse.ToolCalls {
				// Ensure type is set
				toolType := tc.Type
				if toolType == "" {
					toolType = "function"
				}
				toolCalls[i] = ToolCall{
					ID:   fmt.Sprintf("call_%d", i),
					Type: toolType,
					Function: FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
			return toolCalls, "", nil
		}
	}

	// Case 3: Text response returned (conversation finished)
	return nil, choice.Message.Content, nil
}

func (p *Planner) buildSystemPrompt(tools []ToolDef) string {
	var toolList strings.Builder
	for _, tool := range tools {
		toolList.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
	}

	return fmt.Sprintf(`You are a tool-execution agent. Execute user requests using tools.

Available Tools:
%s

RULES:
1. Call tools when user asks you to DO something
2. After getting tool results, respond with a summary - DO NOT call more tools unless needed
3. Stop after 1-2 tool calls and respond to the user
4. ALWAYS read_file before edit_file
5. Use glob to find files by name, grep to search contents

Tool Call Format:
{"tool_calls": [{"type": "function", "function": {"name": "tool_name", "arguments": "{\"param\": \"value\"}"}}]}

Text Response Format:
Just write your response normally (no JSON)

Examples:
User: "Find all go files"
You: [Call glob tool, get results, then respond with summary]

User: "hello"
You: "Hello! How can I help?"

IMPORTANT: After calling tools and getting results, summarize and STOP. Do not keep calling tools.`, toolList.String())
}

func (p *Planner) buildUserPrompt(message string, memories []memory.Memory) string {
	if len(memories) == 0 {
		return message
	}

	var memInfo strings.Builder
	memInfo.WriteString("Relevant Context:\n")
	for i, mem := range memories {
		memInfo.WriteString(fmt.Sprintf("%d. %s\n", i+1, mem.Summary))
	}
	memInfo.WriteString(fmt.Sprintf("\nUser Request:\n%s", message))

	return memInfo.String()
}

// buildToolParameters builds the parameter schema for a tool
func (p *Planner) buildToolParameters(tool types.Tool) map[string]interface{} {
	// Define parameter schemas for known tools
	switch tool.Name() {
	case "shell":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cmd": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			"required": []string{"cmd"},
		}
	case "read_file":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to read (can be relative or absolute)",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Optional line number to start reading from",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Optional number of lines to read",
				},
			},
			"required": []string{"file_path"},
		}
	case "write_file":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to write (can be relative or absolute)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			"required": []string{"file_path", "content"},
		}
	case "edit_file":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to edit (can be relative or absolute)",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "The text to replace",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "The text to replace it with",
				},
				"replace_all": map[string]interface{}{
					"type":        "boolean",
					"description": "Replace all occurrences (default false)",
				},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		}
	case "echo":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The message to echo back",
				},
			},
			"required": []string{"message"},
		}
	case "glob":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The glob pattern to match files (e.g., *.go, **/*.txt)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Optional directory to search in (defaults to working directory)",
			},
		},
		"required": []string{"pattern"},
	}
case "grep":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The regex pattern to search for in file contents",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Optional file or directory to search in (defaults to working directory)",
			},
			"glob": map[string]interface{}{
				"type":        "string",
				"description": "Optional glob pattern to filter files (e.g., *.go, *.txt)",
			},
			"-i": map[string]interface{}{
				"type":        "boolean",
				"description": "Case insensitive search",
			},
			"head_limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results to return",
			},
		},
		"required": []string{"pattern"},
	}
	default:
		return nil
	}
}

// parseResponse handles both tool calls and conversational responses
// Returns (plan, message, error) - exactly one of plan or message will be non-nil
func (p *Planner) parseResponse(content string, toolCalls []ToolCall) (*types.Plan, string, error) {
	// Case 1: LLM returned tool calls → Generate plan from them
	if len(toolCalls) > 0 {
		steps := make([]types.PlanStep, 0, len(toolCalls))

		for _, tc := range toolCalls {
			if tc.Function.Name == "" {
				continue
			}

			// Parse the arguments JSON string
			var args map[string]any
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return nil, "", fmt.Errorf("failed to parse tool arguments: %w", err)
				}
			}

			// Validate tool exists
			if _, ok := p.tools[tc.Function.Name]; !ok {
				return nil, "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
			}

			step := types.PlanStep{
				Tool:        tc.Function.Name,
				Args:        args,
				Description: fmt.Sprintf("Execute %s", tc.Function.Name),
			}
			steps = append(steps, step)
		}

		if len(steps) == 0 {
			return nil, "", fmt.Errorf("no valid tool calls in response")
		}

		plan := &types.Plan{
			Steps:       steps,
			Description: "Execute tools as requested",
			Reasoning:   "User request requires tool execution",
		}
		return plan, "", nil
	}

	// Case 2: LLM returned just text → Conversational response
	// Clean up the content (remove markdown code blocks if present)
	textContent := strings.TrimSpace(content)
	if strings.HasPrefix(textContent, "```json") {
		textContent = strings.TrimPrefix(textContent, "```json")
		textContent = strings.TrimSuffix(textContent, "```")
		textContent = strings.TrimSpace(textContent)
	} else if strings.HasPrefix(textContent, "```") {
		textContent = strings.TrimPrefix(textContent, "```")
		textContent = strings.TrimSuffix(textContent, "```")
		textContent = strings.TrimSpace(textContent)
	}

	// If content is not empty, return it as a conversational response
	if textContent != "" {
		return nil, textContent, nil
	}

	return nil, "", fmt.Errorf("empty response from LLM")
}

// parsePlan is deprecated - use parseResponse instead
// Kept for backward compatibility but should be removed
func (p *Planner) parsePlan(content string) (*types.Plan, error) {
	plan, _, err := p.parseResponse(content, nil)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// Call makes an API call to the LLM
func (c *LLMClient) Call(ctx context.Context, req PlanRequest) (*PlanResponse, error) {
	// Build OpenAI-compatible request
	openAIReq := map[string]interface{}{
		"model":    c.Model,
		"messages": req.Messages,
	}

	if len(req.Tools) > 0 {
		openAIReq["tools"] = req.Tools
	}

	reqJSON, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqJSON)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	// Execute request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var planResp PlanResponse
	if err := json.Unmarshal(body, &planResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &planResp, nil
}

// CallWithMessages makes an API call with flexible message format
func (c *LLMClient) CallWithMessages(ctx context.Context, messages []map[string]interface{}, tools []ToolDef) (*PlanResponse, error) {
	// Build OpenAI-compatible request
	openAIReq := map[string]interface{}{
		"model":    c.Model,
		"messages": messages,
	}

	if len(tools) > 0 {
		openAIReq["tools"] = tools
	}

	reqJSON, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqJSON)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	// Execute request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var planResp PlanResponse
	if err := json.Unmarshal(body, &planResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &planResp, nil
}
