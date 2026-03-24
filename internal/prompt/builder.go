// Package prompt provides dynamic system prompt generation for the agent.
package prompt

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/types"
)

// Mode controls how much context to include in the prompt.
type Mode int

const (
	// ModeFull includes all sections - used for main agent sessions.
	ModeFull Mode = iota
	// ModeMinimal includes only core sections - used for subagents.
	ModeMinimal
	// ModeNone returns only the base identity line.
	ModeNone
)

// Config holds configuration for building the system prompt.
type Config struct {
	// Mode determines which sections to include.
	Mode Mode

	// WorkDir is the current working directory.
	WorkDir string

	// ModelName is the LLM model being used.
	ModelName string

	// Tools available to the agent.
	Tools map[string]types.Tool

	// BootstrapFiles are optional project context files to inject.
	// e.g., "AGENTS.md", "TOOLS.md"
	BootstrapFiles map[string]string // filename -> content

	// SafetyEnabled adds safety guardrails section.
	SafetyEnabled bool

	// Skills lists available skills (optional).
	Skills []string

	// MaxBootstrapSize is the max characters per bootstrap file.
	MaxBootstrapSize int

	// MaxTotalBootstrap is the max total bootstrap content.
	MaxTotalBootstrap int
}

// PromptBuilder assembles dynamic system prompts.
type PromptBuilder struct {
	config Config
}

// NewBuilder creates a new prompt builder with the given configuration.
func NewBuilder(config Config) *PromptBuilder {
	// Set defaults
	if config.MaxBootstrapSize == 0 {
		config.MaxBootstrapSize = 20000
	}
	if config.MaxTotalBootstrap == 0 {
		config.MaxTotalBootstrap = 150000
	}
	return &PromptBuilder{config: config}
}

// Build assembles the complete system prompt.
func (pb *PromptBuilder) Build() string {
	var sb strings.Builder

	// 1. Identity (always included)
	sb.WriteString(pb.buildIdentity())
	sb.WriteString("\n\n")

	if pb.config.Mode == ModeNone {
		return sb.String()
	}

	// 2. Runtime Info
	sb.WriteString(pb.buildRuntime())
	sb.WriteString("\n\n")

	// 3. Tooling
	sb.WriteString(pb.buildTooling())
	sb.WriteString("\n\n")

	if pb.config.Mode == ModeFull {
		// 4. Others (conditional sections)
		others := pb.buildOthers()
		if others != "" {
			sb.WriteString(others)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// buildIdentity creates the base identity section.
func (pb *PromptBuilder) buildIdentity() string {
	return `You are Machinus, an autonomous AI coding agent.

You help users with software engineering tasks: writing code, debugging, refactoring, explaining code, and more.

You think step-by-step, use tools deliberately, and communicate clearly about what you're doing.`
}

// buildRuntime creates the runtime information section.
func (pb *PromptBuilder) buildRuntime() string {
	var sb strings.Builder

	sb.WriteString("## Runtime\n\n")

	// OS and architecture
	sb.WriteString(fmt.Sprintf("- **OS:** %s/%s\n", runtime.GOOS, runtime.GOARCH))

	// Working directory
	if pb.config.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("- **Workspace:** %s\n", pb.config.WorkDir))
	}

	// Model
	if pb.config.ModelName != "" {
		sb.WriteString(fmt.Sprintf("- **Model:** %s\n", pb.config.ModelName))
	}

	// Current time
	sb.WriteString(fmt.Sprintf("- **Time:** %s\n", time.Now().Format("2006-01-02 15:04 MST")))

	return sb.String()
}

// buildTooling creates the tooling section with available tools.
func (pb *PromptBuilder) buildTooling() string {
	var sb strings.Builder

	sb.WriteString("## Tools\n\n")
	sb.WriteString("You have access to these tools:\n\n")

	if len(pb.config.Tools) == 0 {
		sb.WriteString("_No tools available._\n")
		return sb.String()
	}

	// Group tools by category
	categories := pb.categorizeTools()

	for _, cat := range []string{"File Operations", "Search", "Execution", "Network", "Interaction", "Other"} {
		tools, ok := categories[cat]
		if !ok || len(tools) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s\n\n", cat))
		for _, tool := range tools {
			sb.WriteString(fmt.Sprintf("- **%s** - %s\n", tool.Name(), tool.Description()))
		}
		sb.WriteString("\n")
	}

	// Add when-to-use guidance
	sb.WriteString("### When to Use Tools\n\n")
	sb.WriteString(pb.buildToolGuidance())

	return sb.String()
}

// categorizeTools groups tools into logical categories.
func (pb *PromptBuilder) categorizeTools() map[string][]types.Tool {
	categories := make(map[string][]types.Tool)

	for _, tool := range pb.config.Tools {
		cat := pb.getToolCategory(tool.Name())
		categories[cat] = append(categories[cat], tool)
	}

	return categories
}

// getToolCategory returns the category for a tool name.
func (pb *PromptBuilder) getToolCategory(name string) string {
	switch name {
	case "read_file", "write_file", "edit_file", "copy", "move", "delete", "mkdir", "list", "fileinfo":
		return "File Operations"
	case "glob", "grep":
		return "Search"
	case "bash", "shell":
		return "Execution"
	case "http", "websearch", "browser":
		return "Network"
	case "ask_user_input":
		return "Interaction"
	default:
		return "Other"
	}
}

// buildToolGuidance provides when-to-use guidance for tools.
func (pb *PromptBuilder) buildToolGuidance() string {
	var sb strings.Builder

	guidance := []struct {
		tools   []string
		advice  string
	}{
		{
			tools:  []string{"websearch"},
			advice: "Use **websearch** when you need current information, documentation, or research. Don't guess - search.",
		},
		{
			tools:  []string{"read_file"},
			advice: "Use **read_file** before editing to understand existing code structure.",
		},
		{
			tools:  []string{"bash", "shell"},
			advice: "Use **bash** for: git operations, package management, running tests, build commands.",
		},
		{
			tools:  []string{"grep", "glob"},
			advice: "Use **grep** to find text patterns, **glob** to find files by name pattern.",
		},
		{
			tools:  []string{"ask_user_input"},
			advice: "Use **ask_user_input** when you need clarification or confirmation from the user.",
		},
	}

	for _, g := range guidance {
		// Only include if tool is available
		hasTool := false
		for _, t := range g.tools {
			if _, ok := pb.config.Tools[t]; ok {
				hasTool = true
				break
			}
		}
		if hasTool {
			sb.WriteString(g.advice + "\n\n")
		}
	}

	return sb.String()
}

// buildOthers creates the optional sections (safety, skills, bootstrap).
func (pb *PromptBuilder) buildOthers() string {
	var sb strings.Builder

	// Safety section
	if pb.config.SafetyEnabled {
		sb.WriteString("## Safety\n\n")
		sb.WriteString("- Stay focused on the user's task.\n")
		sb.WriteString("- Ask before making destructive changes.\n")
		sb.WriteString("- Don't bypass user oversight.\n\n")
	}

	// Skills section
	if len(pb.config.Skills) > 0 {
		sb.WriteString("## Skills\n\n")
		sb.WriteString("Available skills: " + strings.Join(pb.config.Skills, ", ") + "\n\n")
	}

	// Bootstrap files (project context)
	if len(pb.config.BootstrapFiles) > 0 {
		sb.WriteString(pb.buildBootstrap())
	}

	return sb.String()
}

// buildBootstrap injects project context files.
func (pb *PromptBuilder) buildBootstrap() string {
	var sb strings.Builder

	sb.WriteString("## Project Context\n\n")

	totalChars := 0
	for name, content := range pb.config.BootstrapFiles {
		if content == "" {
			continue
		}

		// Truncate if too large
		truncated := false
		if len(content) > pb.config.MaxBootstrapSize {
			content = content[:pb.config.MaxBootstrapSize]
			truncated = true
		}

		// Check total limit
		if totalChars+len(content) > pb.config.MaxTotalBootstrap {
			break
		}

		sb.WriteString(fmt.Sprintf("### %s\n\n", name))
		sb.WriteString("```\n")
		sb.WriteString(content)
		if truncated {
			sb.WriteString("\n... [truncated]")
		}
		sb.WriteString("\n```\n\n")

		totalChars += len(content)
	}

	return sb.String()
}

// BuildMinimal creates a minimal prompt for subagents.
func BuildMinimal(workDir string, tools map[string]types.Tool) string {
	pb := NewBuilder(Config{
		Mode:    ModeMinimal,
		WorkDir: workDir,
		Tools:   tools,
	})
	return pb.Build()
}

// BuildFull creates a full prompt for the main agent.
func BuildFull(cfg Config) string {
	cfg.Mode = ModeFull
	pb := NewBuilder(cfg)
	return pb.Build()
}
