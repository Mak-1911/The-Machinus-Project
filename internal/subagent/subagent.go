package subagent

import (
	"context"
	"time"

	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/planner"
	"github.com/machinus/cloud-agent/internal/prompt"
	"github.com/machinus/cloud-agent/internal/types"
)

type Subagent struct{
	ID 				string
	orchestrator 	*agent.Orchestrator
	config 			SubagentConfig
	status 	 		Status    // "running" | "completed" | "failed"
	result  		*Result
	startTime		time.Time
}

type LLMConfig struct {
	BaseURL 	string
	APIKey		string
	Model 		string
	WorkDir		string
}

type Status string

const(
	StatusPending	Status = "pending"
	StatusRunning 	Status = "running"
	StatusComplete 	Status = "complete"
	StatusFailed 	Status = "failed"
	StatusTimedOut 	Status = "timed_out"
)

func (s *Subagent) Status() Status {
	return s.status
}

func (s *Subagent) Result() *Result {
	return s.result
}

func New(config SubagentConfig, toolRegistry map[string]types.Tool, llmConfig LLMConfig) *Subagent {
	// Filter tools if specific tools requested
	tools := make(map[string]types.Tool)
	if len(config.Tools) > 0 {
		for _, toolName := range config.Tools {
			if tool, exists := toolRegistry[toolName]; exists {
				tools[toolName] = tool
			}
		}
	} else {
		// Copy all tools
		for k, v := range toolRegistry {
			tools[k] = v
		}
	}

	// Build system prompt
	var systemPrompt string
	if config.SystemPrompt != "" {
		systemPrompt = config.SystemPrompt
	} else {
		// Use dynamic prompt builder with minimal mode for subagents
		cfg := prompt.Config{
			Mode:          prompt.ModeMinimal,
			WorkDir:       llmConfig.WorkDir,
			ModelName:     llmConfig.Model,
			Tools:         tools,
			SafetyEnabled: false, // No safety for subagents
		}
		systemPrompt = prompt.NewBuilder(cfg).Build()
	}

	// Create planner with filtered tools
	p := planner.NewPlannerWithPrompt(
		llmConfig.BaseURL,
		llmConfig.APIKey,
		llmConfig.Model,
		tools,
		systemPrompt,
		nil, // No skills for subagents currently
	)

	// Orchestrator with a fresh session (no history)
	orch := agent.NewOrchestrator(
		p,
		tools,
		nil,    // No store needed for subagents
		nil,    // No session manager
		nil, 	// No memory manager
		nil, 	// No log writer
		"",
	)

	return &Subagent{
		ID: 			config.ID,
		config: 		config,
		orchestrator:	orch,
		status: 		StatusPending,
	}
}

// func Execute() -> Runs the subagent Task
func (s *Subagent) Execute(ctx context.Context) *Result {
	s.startTime = time.Now()
	s.status = StatusRunning

	// Create timeout context
	timeout := s.config.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute via orchestrator
	task, err := s.orchestrator.Execute(ctx, "subagent-user", s.config.Task)

	s.result = &Result{
		ID:   		s.ID,
		Duration: 	time.Since(s.startTime),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			s.status = StatusTimedOut
			s.result.Error = "Subagent Timed out"
		} else {
			s.status = StatusFailed
			s.result.Error = err.Error()
		}
		s.result.Success = false
		return s.result
	}

	// Extract results
	s.status = StatusComplete
	s.result.Success = task.Status == "completed"
	s.result.Summary = task.Response
	s.result.ToolCalls = task.CurrentStep
	s.result.FilesModified = []string{}

	return s.result
}
