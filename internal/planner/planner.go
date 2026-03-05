package planner

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/memory"
	"github.com/machinus/cloud-agent/internal/skills"
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
	client           *LLMClient
	tools            map[string]types.Tool
	cachedXMLPrompt  *SystemPromptXML  // Cached to avoid repeated file reads
	skillsLoader     *skills.Loader    // Skills system
}

// NewPlanner creates a new planner
func NewPlanner(baseURL, apiKey, model string, tools map[string]types.Tool, skillsLoader *skills.Loader) *Planner {
	// Pre-load and cache the XML prompt at initialization
	xmlPrompt := loadSystemPromptXMLStatic()

	return &Planner{
		client: &LLMClient{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
			HTTPClient: &http.Client{
				Timeout: 120 * time.Second, // Increased to 2 minutes for complex queries
			},
		},
		tools:           tools,
		cachedXMLPrompt: xmlPrompt, // Cache at startup
		skillsLoader:    skillsLoader,
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

// SystemPromptXML represents the XML structure for system prompts
type SystemPromptXML struct {
	XMLName          xml.Name           `xml:"system_prompt"`
	Role             string             `xml:"role"`
	ExecutionStrategy ExecutionStrategy  `xml:"execution_strategy"`
	ToolCallFormat   ToolCallFormat     `xml:"tool_call_format"`
	ImportantGuidelines ImportantGuidelines `xml:"important_guidelines"`
	ErrorRecovery    ErrorRecovery      `xml:"error_recovery"`
	Examples         Examples           `xml:"examples"`
}

type ExecutionStrategy struct {
	Steps []Step `xml:"step"`
}

type Step struct {
	Order int    `xml:"order,attr"`
	Text  string `xml:",chardata"`
}

type ToolCallFormat struct {
	Description string   `xml:"description"`
	Example     string   `xml:"example"`
}

type ImportantGuidelines struct {
	Guidelines []Guideline `xml:"guideline"`
}

type Guideline struct {
	Text string `xml:",chardata"`
}

type ErrorRecovery struct {
	Title             string            `xml:"title"`
	Description       string            `xml:"description"`
	FailureTypes      FailureTypes      `xml:"failure_types"`
	RetryStrategies   RetryStrategies   `xml:"retry_strategies"`
	SuggestedAlternatives SuggestedAlternatives `xml:"suggested_alternatives"`
	WhenToStop        WhenToStop        `xml:"when_to_stop"`
	TransparentCommunication TransparentCommunication `xml:"transparent_communication"`
	ExecutionHistory  ExecutionHistory  `xml:"execution_history"`
}

type FailureTypes struct {
	Types []FailureType `xml:"type"`
}

type FailureType struct {
	Name     string   `xml:"name,attr"`
	Description string `xml:"description"`
	Examples string   `xml:"examples"`
}

type RetryStrategies struct {
	Strategies []RetryStrategy `xml:"strategy"`
}

type RetryStrategy struct {
	Name      string    `xml:"name,attr"`
	Description string `xml:"description"`
	Attempts  []Attempt `xml:"attempts>attempt"`
	Fallback  string    `xml:"fallback"`
	Message   string    `xml:"message"`
}

type Attempt struct {
	Number string `xml:"number,attr"`
	Text   string `xml:",chardata"`
}

type SuggestedAlternatives struct {
	Alternatives []Alternative `xml:"alternative"`
}

type Alternative struct {
	Text string `xml:",chardata"`
}

type WhenToStop struct {
	Rules []Rule `xml:"rule"`
}

type Rule struct {
	Text string `xml:",chardata"`
}

type TransparentCommunication struct {
	Description string   `xml:"description"`
	Examples    []Example `xml:"examples>example"`
}

type ExecutionHistory struct {
	Description string `xml:"description"`
	Items       []Item  `xml:"item"`
}

type Item struct {
	Text string `xml:",chardata"`
}

type Examples struct {
	Examples []Example `xml:"example"`
}

type Example struct {
	User      string `xml:"user"`
	Response  string `xml:"response"`
	Attempt   string `xml:"attempt"`
	Text      string `xml:",chardata"`
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
	// Use cached XML prompt if available, otherwise fallback
	if p.cachedXMLPrompt != nil {
		return p.buildPromptFromXML(p.cachedXMLPrompt, tools)
	}

	// Fallback to hardcoded prompt if XML not cached
	return p.buildFallbackSystemPrompt(tools)
}

// loadSystemPromptXMLStatic loads and parses the system prompt XML file (static version for initialization)
func loadSystemPromptXMLStatic() *SystemPromptXML {
	// Try multiple possible paths
	paths := []string{
		"prompts/agent_system.xml",
		"./prompts/agent_system.xml",
		"../prompts/agent_system.xml",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			var prompt SystemPromptXML
			if err := xml.Unmarshal(data, &prompt); err == nil {
				return &prompt
			}
		}
	}

	// Return nil if loading fails - will use hardcoded fallback
	return nil
}

// buildPromptFromXML builds the system prompt from cached XML
func (p *Planner) buildPromptFromXML(xmlPrompt *SystemPromptXML, tools []ToolDef) string {
	var prompt strings.Builder

	// Role
	prompt.WriteString(fmt.Sprintf("%s\n\n", xmlPrompt.Role))

	// Tools section
	prompt.WriteString("# Available Tools\n")
	for _, tool := range tools {
		// Get the actual tool instance to access metadata
		toolInstance, exists := p.tools[tool.Function.Name]
		if exists {
			// Include rich metadata
			examples := toolInstance.Examples()
			whenToUse := toolInstance.WhenToUse()
			chainsWith := toolInstance.ChainsWith()

			prompt.WriteString(fmt.Sprintf("\n## %s\n", tool.Function.Name))
			prompt.WriteString(fmt.Sprintf("Description: %s\n", tool.Function.Description))

			if whenToUse != "" {
				prompt.WriteString(fmt.Sprintf("When to use: %s\n", whenToUse))
			}

			if len(chainsWith) > 0 {
				prompt.WriteString(fmt.Sprintf("Works well with: %s\n", strings.Join(chainsWith, ", ")))
			}

			if len(examples) > 0 {
				prompt.WriteString("Examples:\n")
				for i, example := range examples {
					prompt.WriteString(fmt.Sprintf("  %d. %s\n", i+1, example.Description))
					// Format args as JSON
					if argsJSON, err := json.Marshal(example.Input); err == nil {
						prompt.WriteString(fmt.Sprintf("     Args: %s\n", string(argsJSON)))
					}
				}
			}
			prompt.WriteString("\n")
		} else {
			// Fallback if tool instance not found
			prompt.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
		}
	}

	// Skills section (NEW)
	if p.skillsLoader != nil {
		prompt.WriteString("\n# Available Skills\n")
		prompt.WriteString(p.skillsLoader.GetAvailableSkillsXML())
		prompt.WriteString("\n")
	}

	// Execution Strategy
	prompt.WriteString("\n# Execution Strategy\n")
	for _, step := range xmlPrompt.ExecutionStrategy.Steps {
		prompt.WriteString(fmt.Sprintf("%d. %s\n", step.Order, step.Text))
	}
	prompt.WriteString("\n")

	// Tool Call Format
	prompt.WriteString("# Tool Call Format\n")
	prompt.WriteString(fmt.Sprintf("%s\n", xmlPrompt.ToolCallFormat.Description))
	prompt.WriteString(xmlPrompt.ToolCallFormat.Example)
	prompt.WriteString("\n")

	// Important Guidelines
	prompt.WriteString("# Important\n")
	for _, guideline := range xmlPrompt.ImportantGuidelines.Guidelines {
		prompt.WriteString(fmt.Sprintf("- %s\n", guideline.Text))
	}
	prompt.WriteString("\n")

	// Error Recovery Section
	prompt.WriteString(fmt.Sprintf("## %s\n", xmlPrompt.ErrorRecovery.Title))
	prompt.WriteString(fmt.Sprintf("%s\n\n", xmlPrompt.ErrorRecovery.Description))

	// Failure Types
	prompt.WriteString("### 1. Identify Failure Type\n")
	prompt.WriteString("Tool errors include metadata about the failure type:\n")
	for _, ft := range xmlPrompt.ErrorRecovery.FailureTypes.Types {
		prompt.WriteString(fmt.Sprintf("- %s: %s\n", ft.Name, ft.Description))
		if ft.Examples != "" {
			prompt.WriteString(fmt.Sprintf("  %s\n", ft.Examples))
		}
	}
	prompt.WriteString("\n")

	// Retry Strategies
	prompt.WriteString("### 2. Adaptive Retry Strategies\n")
	prompt.WriteString("Don't just repeat the same call - adapt:\n\n")
	for _, strategy := range xmlPrompt.ErrorRecovery.RetryStrategies.Strategies {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", strings.Title(strategy.Name), strategy.Description))
		for _, attempt := range strategy.Attempts {
			prompt.WriteString(fmt.Sprintf("- %s\n", attempt.Text))
		}
		if strategy.Fallback != "" {
			prompt.WriteString(fmt.Sprintf("Fallback: %s\n", strategy.Fallback))
		}
		if strategy.Message != "" {
			prompt.WriteString(fmt.Sprintf("Message: \"%s\"\n", strategy.Message))
		}
		prompt.WriteString("\n")
	}

	// Suggested Alternatives
	prompt.WriteString("### 3. Suggested Alternatives\n")
	prompt.WriteString("When tools fail, they may suggest alternatives:\n")
	for _, alt := range xmlPrompt.ErrorRecovery.SuggestedAlternatives.Alternatives {
		prompt.WriteString(fmt.Sprintf("- %s\n", alt.Text))
	}
	prompt.WriteString("\n")

	// When to Stop
	prompt.WriteString("### 4. Know When to Stop\n")
	for _, rule := range xmlPrompt.ErrorRecovery.WhenToStop.Rules {
		prompt.WriteString(fmt.Sprintf("- %s\n", rule.Text))
	}
	prompt.WriteString("\n")

	// Transparent Communication
	prompt.WriteString("### 5. Transparent Communication\n")
	prompt.WriteString(fmt.Sprintf("%s\n", xmlPrompt.ErrorRecovery.TransparentCommunication.Description))
	prompt.WriteString("Examples:\n")
	for _, ex := range xmlPrompt.ErrorRecovery.TransparentCommunication.Examples {
		prompt.WriteString(fmt.Sprintf("- \"%s\"\n", ex.Text))
	}
	prompt.WriteString("\n")

	// Execution History
	prompt.WriteString("### 6. Use Execution History\n")
	prompt.WriteString(fmt.Sprintf("%s\n", xmlPrompt.ErrorRecovery.ExecutionHistory.Description))
	for _, item := range xmlPrompt.ErrorRecovery.ExecutionHistory.Items {
		prompt.WriteString(fmt.Sprintf("- %s\n", item.Text))
	}
	prompt.WriteString("\n")

	// Examples
	prompt.WriteString("# Examples\n")
	for _, ex := range xmlPrompt.Examples.Examples {
		if ex.User != "" {
			prompt.WriteString(fmt.Sprintf("User: \"%s\"\n", ex.User))
			if ex.Response != "" {
				prompt.WriteString(fmt.Sprintf("You: %s\n", ex.Response))
			}
		}
		if ex.Attempt != "" {
			prompt.WriteString(fmt.Sprintf("%s\n", ex.Attempt))
		}
		if ex.Text != "" {
			prompt.WriteString(fmt.Sprintf("%s\n", ex.Text))
		}
		prompt.WriteString("\n")
	}

	return prompt.String()
}

// formatXMLPrompt formats the loaded XML prompt with tool list
func (p *Planner) formatXMLPrompt(xmlPrompt *SystemPromptXML, toolList string) string {
	var prompt strings.Builder

	// Role
	prompt.WriteString(fmt.Sprintf("%s\n\n", xmlPrompt.Role))

	// Tools section
	prompt.WriteString("# Available Tools\n")
	prompt.WriteString(toolList)
	prompt.WriteString("\n")

	// Execution Strategy
	prompt.WriteString("# Execution Strategy\n")
	for _, step := range xmlPrompt.ExecutionStrategy.Steps {
		prompt.WriteString(fmt.Sprintf("%d. %s\n", step.Order, step.Text))
	}
	prompt.WriteString("\n")

	// Tool Call Format
	prompt.WriteString("# Tool Call Format\n")
	prompt.WriteString(fmt.Sprintf("%s\n", xmlPrompt.ToolCallFormat.Description))
	prompt.WriteString(xmlPrompt.ToolCallFormat.Example)
	prompt.WriteString("\n")

	// Important Guidelines
	prompt.WriteString("# Important\n")
	for _, guideline := range xmlPrompt.ImportantGuidelines.Guidelines {
		prompt.WriteString(fmt.Sprintf("- %s\n", guideline.Text))
	}
	prompt.WriteString("\n")

	// Error Recovery Section
	prompt.WriteString(fmt.Sprintf("## %s\n", xmlPrompt.ErrorRecovery.Title))
	prompt.WriteString(fmt.Sprintf("%s\n\n", xmlPrompt.ErrorRecovery.Description))

	// Failure Types
	prompt.WriteString("### 1. Identify Failure Type\n")
	prompt.WriteString("Tool errors include metadata about the failure type:\n")
	for _, ft := range xmlPrompt.ErrorRecovery.FailureTypes.Types {
		prompt.WriteString(fmt.Sprintf("- %s: %s\n", ft.Name, ft.Description))
		if ft.Examples != "" {
			prompt.WriteString(fmt.Sprintf("  %s\n", ft.Examples))
		}
	}
	prompt.WriteString("\n")

	// Retry Strategies
	prompt.WriteString("### 2. Adaptive Retry Strategies\n")
	prompt.WriteString("Don't just repeat the same call - adapt:\n\n")
	for _, strategy := range xmlPrompt.ErrorRecovery.RetryStrategies.Strategies {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", strings.Title(strategy.Name), strategy.Description))
		for _, attempt := range strategy.Attempts {
			prompt.WriteString(fmt.Sprintf("- %s\n", attempt.Text))
		}
		if strategy.Fallback != "" {
			prompt.WriteString(fmt.Sprintf("Fallback: %s\n", strategy.Fallback))
		}
		if strategy.Message != "" {
			prompt.WriteString(fmt.Sprintf("Message: \"%s\"\n", strategy.Message))
		}
		prompt.WriteString("\n")
	}

	// Suggested Alternatives
	prompt.WriteString("### 3. Suggested Alternatives\n")
	prompt.WriteString("When tools fail, they may suggest alternatives:\n")
	for _, alt := range xmlPrompt.ErrorRecovery.SuggestedAlternatives.Alternatives {
		prompt.WriteString(fmt.Sprintf("- %s\n", alt.Text))
	}
	prompt.WriteString("\n")

	// When to Stop
	prompt.WriteString("### 4. Know When to Stop\n")
	for _, rule := range xmlPrompt.ErrorRecovery.WhenToStop.Rules {
		prompt.WriteString(fmt.Sprintf("- %s\n", rule.Text))
	}
	prompt.WriteString("\n")

	// Transparent Communication
	prompt.WriteString("### 5. Transparent Communication\n")
	prompt.WriteString(fmt.Sprintf("%s\n", xmlPrompt.ErrorRecovery.TransparentCommunication.Description))
	prompt.WriteString("Examples:\n")
	for _, ex := range xmlPrompt.ErrorRecovery.TransparentCommunication.Examples {
		prompt.WriteString(fmt.Sprintf("- \"%s\"\n", ex.Text))
	}
	prompt.WriteString("\n")

	// Execution History
	prompt.WriteString("### 6. Use Execution History\n")
	prompt.WriteString(fmt.Sprintf("%s\n", xmlPrompt.ErrorRecovery.ExecutionHistory.Description))
	for _, item := range xmlPrompt.ErrorRecovery.ExecutionHistory.Items {
		prompt.WriteString(fmt.Sprintf("- %s\n", item.Text))
	}
	prompt.WriteString("\n")

	// Examples
	prompt.WriteString("# Examples\n")
	for _, ex := range xmlPrompt.Examples.Examples {
		if ex.User != "" {
			prompt.WriteString(fmt.Sprintf("User: \"%s\"\n", ex.User))
			if ex.Response != "" {
				prompt.WriteString(fmt.Sprintf("You: %s\n", ex.Response))
			}
		}
		if ex.Attempt != "" {
			prompt.WriteString(fmt.Sprintf("%s\n", ex.Attempt))
		}
		if ex.Text != "" {
			prompt.WriteString(fmt.Sprintf("%s\n", ex.Text))
		}
		prompt.WriteString("\n")
	}

	return prompt.String()
}

// buildFallbackSystemPrompt returns the hardcoded prompt as fallback
func (p *Planner) buildFallbackSystemPrompt(tools []ToolDef) string {
	var toolList strings.Builder
	for _, tool := range tools {
		// Get the actual tool instance to access metadata
		toolInstance, exists := p.tools[tool.Function.Name]
		if exists {
			// Include rich metadata
			examples := toolInstance.Examples()
			whenToUse := toolInstance.WhenToUse()
			chainsWith := toolInstance.ChainsWith()

			toolList.WriteString(fmt.Sprintf("\n## %s\n", tool.Function.Name))
			toolList.WriteString(fmt.Sprintf("Description: %s\n", tool.Function.Description))

			if whenToUse != "" {
				toolList.WriteString(fmt.Sprintf("When to use: %s\n", whenToUse))
			}

			if len(chainsWith) > 0 {
				toolList.WriteString(fmt.Sprintf("Works well with: %s\n", strings.Join(chainsWith, ", ")))
			}

			if len(examples) > 0 {
				toolList.WriteString("Examples:\n")
				for i, example := range examples {
					toolList.WriteString(fmt.Sprintf("  %d. %s\n", i+1, example.Description))
					// Format args as JSON
					if argsJSON, err := json.Marshal(example.Input); err == nil {
						toolList.WriteString(fmt.Sprintf("     Args: %s\n", string(argsJSON)))
					}
				}
			}
			toolList.WriteString("\n")
		} else {
			// Fallback if tool instance not found
			toolList.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
		}
	}

	prompt := `You are an intelligent agent that executes user requests by calling tools.

# Available Tools
` + toolList.String() + `

# Execution Strategy
1. Understand the user's goal
2. Select the appropriate tool(s) based on the task
3. Chain tools when needed (e.g., glob -> grep -> read_file)
4. After 1-2 tool calls, summarize results and STOP
5. Always read files before editing them
6. Use glob to find files, grep to search contents

# Tool Call Format
Return JSON with tool_calls array:
{
  "tool_calls": [{
    "type": "function",
    "function": {
      "name": "tool_name",
      "arguments": "{\"param\": \"value\"}"
    }
  }]
}

# Important
- Read BEFORE editing (use read_file before edit_file)
- Chain related tools together (see "Works well with" above)
- After getting results, provide a clear summary
- STOP after completing the task - don't keep calling tools unnecessarily

## Error Recovery & Retry Strategy (Claude Code Style)

When a tool fails, analyze the error before retrying:

### 1. Identify Failure Type
Tool errors include metadata about the failure type:
- Hard failures: Don't retry (404, permission denied, auth failed)
- Soft failures: Retry with backoff (timeout, 503, rate limit)
- Partial failures: Consider using partial results
- Ambiguous failures: Use context to decide

### 2. Adaptive Retry Strategies
Don't just repeat the same call - adapt:

Timeout errors: Increase timeout duration
- Attempt 1: http tool with 30s timeout -> timeout
- Attempt 2: http tool with 60s timeout -> success

Too many results: Be more specific
- Attempt 1: grep with broad pattern -> too many matches
- Attempt 2: grep with more specific pattern -> better results
- Attempt 3: Use code tool with symbol_search -> precise results

File not found: Search for alternatives
- Attempt 1: read_file specific path -> not found
- Attempt 2: glob to find similar files -> found alternatives
- Attempt 3: Use closest match or ask user

Permission errors: Try alternative approach
- Attempt 1: read protected file -> permission denied
- Attempt 2: Ask user to provide content or use different approach

Tool unavailable: Try alternative tools
- Attempt 1: browser tool -> service not running
- Fallback: Use http tool for basic content
- Message: "Browser unavailable, using HTTP fetch"

### 3. Suggested Alternatives
When tools fail, they may suggest alternatives:
- HTTP failures -> Try browser tool
- File too large -> Stream with shell (head/tail)
- Search too broad -> Use code/LSP tools
- Network issues -> Try different endpoint

### 4. Know When to Stop
- Don't retry auth failures
- Don't retry 404s (file doesn't exist)
- Don't retry syntax errors in your own commands
- DO retry timeouts and 5xx errors
- Ask user for help if stuck after 2-3 attempts

### 5. Transparent Communication
When retries happen, explain what happened:
- "HTTP request timed out (attempt 1/3). Retrying with increased timeout..."
- "File too large (50MB). Using shell to stream first 1000 lines..."
- "Browser unavailable. Using HTTP fetch as fallback."

### 6. Use Execution History
Tool results include execution history showing:
- Number of attempts made
- What failed and why
- How long each attempt took
- Whether alternatives were tried

# Examples
User: "Find all Go files"
You: Use glob tool with pattern "*.go", then summarize results

User: "Search for TODO comments"
You: Use grep tool with pattern "TODO" in "*.go" files

User: "What files do we have?"
You: Use list tool to show directory contents

User: "Fetch https://example.com/api/data"
Attempt 1: http tool with 30s timeout - timed out
You: "Request timed out. Retrying with 60s timeout..."
Attempt 2: http tool with 60s timeout - success

User: "hello"
You: "Hello! I'm here to help with file operations, web requests, and system tasks."
`
	return prompt
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
	case "http":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The HTTP endpoint to call (must start with http:// or https://)",
			},
			"method": map[string]interface{}{
				"type":        "string",
				"description": "HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)",
				"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "Optional HTTP headers (e.g., {\"Authorization\": \"Bearer token\"})",
			},
			"body": map[string]interface{}{
				"type":        "object",
				"description": "Optional request body for POST/PUT/PATCH (will be JSON encoded)",
			},
			"query_params": map[string]interface{}{
				"type":        "object",
				"description": "Optional query parameters to append to URL",
			},
		},
		"required": []string{"url", "method"},
	}
	case "copy":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"src": map[string]interface{}{
				"type":        "string",
				"description": "Source file or directory path to copy from",
			},
			"dest": map[string]interface{}{
				"type":        "string",
				"description": "Destination path to copy to",
			},
			"overwrite": map[string]interface{}{
				"type":        "boolean",
				"description": "Overwrite if destination exists (default: false)",
			},
		},
		"required": []string{"src", "dest"},
	}
	case "move":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"src": map[string]interface{}{
				"type":        "string",
				"description": "Source file or directory path to move",
			},
			"dest": map[string]interface{}{
				"type":        "string",
				"description": "Destination path (can be a rename or different directory)",
			},
			"overwrite": map[string]interface{}{
				"type":        "boolean",
				"description": "Overwrite if destination exists (default: false)",
			},
		},
		"required": []string{"src", "dest"},
	}
	case "delete":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory path to delete",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "Delete directories recursively (default: false)",
			},
		},
		"required": []string{"path"},
	}
	case "list":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path to list (default: current directory)",
			},
			"details": map[string]interface{}{
				"type":        "boolean",
				"description": "Show detailed information (size, permissions, dates)",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "List subdirectories recursively",
			},
			"sort": map[string]interface{}{
				"type":        "string",
				"description": "Sort by 'name', 'size', or 'date'",
				"enum":        []string{"name", "size", "date"},
			},
		},
		"required": []string{},
	}
	case "mkdir":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path to create",
			},
			"parents": map[string]interface{}{
				"type":        "boolean",
				"description": "Create parent directories as needed (like mkdir -p)",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "Optional permissions (e.g., '0755' for rwxr-xr-x)",
			},
		},
		"required": []string{"path"},
	}
	case "fileinfo":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory path to get information about",
			},
			"include_mime": map[string]interface{}{
				"type":        "boolean",
				"description": "Include MIME type detection for files",
			},
		},
		"required": []string{"path"},
	}
	case "browser":
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"actions": map[string]interface{}{
				"type":        "array",
				"description": "List of browser actions to execute in sequence",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"description": "Action to perform",
							"enum":        []string{"create_instance", "navigate", "goto", "snapshot", "click", "fill", "text", "screenshot", "close"},
						},
						"profile": map[string]interface{}{
							"type":        "string",
							"description": "Profile name for browser instance (for 'create_instance' action, default 'default')",
						},
						"url": map[string]interface{}{
							"type":        "string",
							"description": "URL to navigate to (for 'navigate' or 'goto' action)",
						},
						"filter": map[string]interface{}{
							"type":        "string",
							"description": "Filter for snapshot (for 'snapshot' action): 'interactive', 'all', 'visible' (default 'interactive')",
						},
						"ref": map[string]interface{}{
							"type":        "string",
							"description": "Element reference from accessibility tree (for 'click' and 'fill' actions)",
						},
						"value": map[string]interface{}{
							"type":        "string",
							"description": "Value to fill (for 'fill' action)",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path for screenshot (for 'screenshot' action)",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		"required": []string{"actions"},
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
