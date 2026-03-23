// Package app provides tool execution for the LLM agent.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/tools"
	"github.com/machinus/cloud-agent/internal/types"
)

// ToolExecutor handles tool registration and execution for the LLM agent.
type ToolExecutor struct {
	tools  map[string]types.Tool
	workDir string
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor(workDir string) *ToolExecutor {
	te := &ToolExecutor{
		tools:   make(map[string]types.Tool),
		workDir: workDir,
	}
	te.registerDefaultTools()
	return te
}

// registerDefaultTools registers the default set of tools.
// This should match the CLI tools in cmd/cli/main.go::initializeTools
func (te *ToolExecutor) registerDefaultTools() {
	// Shell/Command execution
	te.RegisterTool("bash", tools.NewShellTool(te.workDir, 30*time.Second, true))
	te.RegisterTool("shell", tools.NewShellTool(te.workDir, 30*time.Second, true))

	// File tools (using same implementations as CLI)
	te.RegisterTool("read_file", tools.NewFileReadTool(10*1024*1024))
	te.RegisterTool("write_file", tools.NewFileWriteTool(10*1024*1024))
	te.RegisterTool("edit_file", tools.NewFileEditTool(10*1024*1024))

	// Search tools
	te.RegisterTool("glob", tools.NewGlobTool(1000))
	te.RegisterTool("grep", tools.NewGrepTool(1000))

	// File operations
	te.RegisterTool("copy", tools.NewCopyTool(1))
	te.RegisterTool("move", tools.NewMoveTool())
	te.RegisterTool("delete", tools.NewDeleteTool(false))
	te.RegisterTool("list", tools.NewListTool())
	te.RegisterTool("mkdir", tools.NewMakeDirectoryTool())
	te.RegisterTool("fileinfo", tools.NewFileInfoTool())

	// HTTP requests
	te.RegisterTool("http", tools.NewHTTPTool(30, 10)) // 30s timeout, 10MB max response

	// Web search - CRITICAL for research tasks
	te.RegisterTool("websearch", tools.NewWebSearchTool(30, 10))

	// Browser automation (PinchTab)
	pinchtabURL := os.Getenv("PINCHTAB_URL")
	if pinchtabURL == "" {
		pinchtabURL = "http://localhost:9867"
	}
	te.RegisterTool("browser", tools.NewPinchTabTool(pinchtabURL))

	// User interaction
	te.RegisterTool("ask_user_input", tools.NewAskUserInputTool(300))
}

// RegisterTool registers a tool with the executor.
func (te *ToolExecutor) RegisterTool(name string, tool types.Tool) {
	te.tools[name] = tool
}

// GetTools returns all registered tools.
func (te *ToolExecutor) GetTools() map[string]types.Tool {
	return te.tools
}

// GetTool returns a tool by name.
func (te *ToolExecutor) GetTool(name string) (types.Tool, bool) {
	tool, ok := te.tools[name]
	return tool, ok
}

// Execute executes a tool call and returns the result.
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any) (types.ToolResult, error) {
	tool, ok := te.tools[toolName]
	if !ok {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}

	// Validate arguments
	if err := tool.ValidateArgs(args); err != nil {
		return types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid arguments: %v", err),
		}, nil
	}

	// Execute the tool
	return tool.Execute(ctx, args)
}

// GetToolsPrompt returns a prompt description of available tools for the LLM.
func (te *ToolExecutor) GetToolsPrompt() string {
	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("You have access to these tools. Only use them when the user's request requires it:\n\n")

	for name, tool := range te.tools {
		sb.WriteString(fmt.Sprintf("### %s\n", name))
		sb.WriteString(fmt.Sprintf("%s\n", tool.Description()))

		// Add examples if available
		if examples := tool.Examples(); len(examples) > 0 {
			sb.WriteString("\nExamples:\n")
			for _, ex := range examples {
				argsJSON, _ := json.Marshal(ex.Input)
				sb.WriteString(fmt.Sprintf("- %s: %s\n", ex.Description, string(argsJSON)))
			}
		}

		if when := tool.WhenToUse(); when != "" {
			sb.WriteString(fmt.Sprintf("\nWhen to use: %s\n", when))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Tool Call Format\n\n")
	sb.WriteString("When you need to use a tool:\n\n")
	sb.WriteString("Format 1 - Multi-line:\n")
	sb.WriteString("```\n")
	sb.WriteString("tool:tool_name\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "arg1": "value1",` + "\n")
	sb.WriteString(`  "arg2": "value2"` + "\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Format 2 - Single-line:\n")
	sb.WriteString("```\n")
	sb.WriteString("tool:tool_name {\"arg1\": \"value1\"}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("REMINDER: Use the 'tool:' prefix and provide valid JSON arguments.\n\n")

	sb.WriteString("For bash/shell commands:\n\n")
	sb.WriteString("```\nbash\nyour command here\n```\n")
	sb.WriteString("or\n\n")
	sb.WriteString("```\nshell\nyour command here\n```\n\n")

	return sb.String()
}

