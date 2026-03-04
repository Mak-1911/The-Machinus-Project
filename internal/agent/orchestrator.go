package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/machinus/cloud-agent/internal/memory"
	"github.com/machinus/cloud-agent/internal/planner"
	"github.com/machinus/cloud-agent/internal/types"
)

// Orchestrator coordinates the planning and execution of tasks
type Orchestrator struct {
	planner      	*planner.Planner
	tools        	map[string]types.Tool
	memory       	*memory.Manager
	store        	Store
	logWriter    	LogWriter
	sessionManager *SessionManager
	sessionID 		string
}

// Store defines the storage interface for the orchestrator
type Store interface {
	SaveTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, taskID string) (*Task, error)
	SaveTaskLog(ctx context.Context, log *TaskLog) error
	GetTaskLogs(ctx context.Context, taskID string, limit int) ([]TaskLog, error)
}

// LogWriter defines the interface for writing logs
type LogWriter interface {
	Write(ctx context.Context, taskID string, level, message string, step int) error
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(
	p *planner.Planner,
	tools map[string]types.Tool,
	m *memory.Manager,
	store Store,
	logWriter LogWriter,
	sessionMgr *SessionManager,
	sessionID string,
) *Orchestrator {
	return &Orchestrator{
		planner:   		p,
		tools:     		tools,
		memory:    		m,
		store:     		store,
		logWriter: 		logWriter,
		sessionManager: sessionMgr,
		sessionID: 		sessionID,
	}
}

// executeWithRetry implements Claude Code's retry strategy
func (o *Orchestrator) executeWithRetry(ctx context.Context, tool types.Tool, toolName string, args map[string]any, taskID string, step int) types.ToolResult {
	maxAttempts := 3
	policy := types.DefaultRetryPolicy()
	var history types.ExecutionHistory
	history.ToolName = toolName

	for attemptNum := 0; attemptNum < maxAttempts; attemptNum++ {
		startTime := time.Now()
		result := types.ToolResult{}

		// Execute the tool
		result, _ = tool.Execute(ctx, args)
		duration := time.Since(startTime)

		// Track this attempt
		execAttempt := types.ExecutionAttempt{
			Tool:          toolName,
			Args:          args,
			AttemptNumber: attemptNum + 1,
			Duration:      duration.Milliseconds(),
			Metadata:      map[string]any{},
		}

		if result.Error != "" {
			execAttempt.Error = result.Error
			execAttempt.FailureType = result.FailureType
		}

		if result.Data != nil && result.Progress > 0 {
			execAttempt.PartialResult = result.Data
		}

		history.Attempts = append(history.Attempts, execAttempt)
		history.Duration += duration.Milliseconds()

		// If successful, return immediately
		if result.Success {
			history.Success = true
			o.logWriter.Write(ctx, taskID, "info", fmt.Sprintf("Tool succeeded on attempt %d", attemptNum+1), step)
			// Add execution history to result data
			if dataMap, ok := result.Data.(map[string]any); ok {
				dataMap["execution_history"] = history
			} else {
				result.Data = map[string]any{"execution_history": history}
			}
			return result
		}

		// Check if we should retry
		if !result.Retryable {
			o.logWriter.Write(ctx, taskID, "warning", fmt.Sprintf("Tool failed with non-retryable error: %s", result.Error), step)
			history.Success = false
			if dataMap, ok := result.Data.(map[string]any); ok {
				dataMap["execution_history"] = history
			} else {
				result.Data = map[string]any{"execution_history": history}
			}
			return result
		}

		// Last attempt - don't retry
		if attemptNum == maxAttempts-1 {
			o.logWriter.Write(ctx, taskID, "warning", fmt.Sprintf("Tool failed after %d attempts: %s", maxAttempts, result.Error), step)
			history.Success = false
			if dataMap, ok := result.Data.(map[string]any); ok {
				dataMap["execution_history"] = history
			} else {
				result.Data = map[string]any{"execution_history": history}
			}
			return result
		}

		// Log retry attempt
		backoffMs := policy.InitialBackoff * (1 << uint(attemptNum)) // Exponential backoff
		if backoffMs > policy.MaxBackoff {
			backoffMs = policy.MaxBackoff
		}

		o.logWriter.Write(ctx, taskID, "info", fmt.Sprintf("Attempt %d failed, retrying in %dms: %s", attemptNum+1, backoffMs, result.Error), step)

		// Adaptive retry: Try alternative tools if suggested
		if len(result.Alternatives) > 0 && attemptNum == 1 {
			altToolName := result.Alternatives[0]
			if altTool, ok := o.tools[altToolName]; ok {
				o.logWriter.Write(ctx, taskID, "info", fmt.Sprintf("Trying alternative tool: %s", altToolName), step)
				// Try alternative tool (simple substitution - in future, transform args appropriately)
				altResult := o.executeWithRetry(ctx, altTool, altToolName, args, taskID, step)
				if altDataMap, ok := altResult.Data.(map[string]any); ok {
					altDataMap["original_tool"] = toolName
					altDataMap["used_alternative"] = true
				}
				return altResult
			}
		}

		// Adaptive retry: Modify parameters based on failure type
		if result.FailureType == types.FailureTypeSoft {
			// For timeouts, increase timeout if available
			if timeoutVal, ok := args["timeout"].(int); ok {
				args["timeout"] = timeoutVal * 2
				o.logWriter.Write(ctx, taskID, "info", fmt.Sprintf("Increased timeout to %d", args["timeout"]), step)
			}
		}

		// Wait before retry with exponential backoff
		select {
		case <-time.After(time.Duration(backoffMs) * time.Millisecond):
			// Continue to retry
		case <-ctx.Done():
			// Context cancelled
			result.Error = "retry cancelled by context"
			result.FailureType = types.FailureTypeSoft
			result.Retryable = false
			history.Success = false
			if dataMap, ok := result.Data.(map[string]any); ok {
				dataMap["execution_history"] = history
			} else {
				result.Data = map[string]any{"execution_history": history}
			}
			return result
		}
	}

	// Shouldn't reach here, but return last result
	history.Success = false
	return types.ToolResult{
		Success:     false,
		Error:       "maximum retry attempts exceeded",
		FailureType: types.FailureTypeSoft,
		Retryable:   false,
		Data:        map[string]any{"execution_history": history},
	}
}

// Execute handles a user request from start to finish
func (o *Orchestrator) Execute(ctx context.Context, userID, message string) (*Task, error) {
	// Create task
	task := &Task{
		ID:        uuid.New().String(),
		UserID:    userID,
		Message:   message,
		Status:    "planning",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save initial task state
	if err := o.store.SaveTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	// Retrieve relevant memories
	memories, err := o.memory.Retrieve(ctx, userID, message)
	if err != nil {
		o.logWriter.Write(ctx, task.ID, "warning", fmt.Sprintf("Failed to retrieve memories: %v", err), 0)
	}

	// Load conversation history from session
	conversation := []planner.ConversationMessage{}

	if o.sessionManager != nil && o.sessionID != "" {
		session, err := o.sessionManager.GetSession(ctx, o.sessionID)
		if err == nil && session != nil && len(session.Messages) > 0 {
			// Convert session messages to planner format
			for _, msg := range session.Messages {
				conversation = append(conversation, planner.ConversationMessage{
					Role:    msg.Role,
					Content: msg.Content,
					ToolID:  msg.ToolID,
				})
			}
			o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Loaded %d messages from session", len(session.Messages)), 0)
		}
	}

	// Add new user message
	conversation = append(conversation, planner.ConversationMessage{
		Role:    "user",
		Content: message,
	})

	// Continuation loop - keep executing tools until LLM returns final text
	o.logWriter.Write(ctx, task.ID, "info", "Processing request...", 0)
	task.Status = "executing"
	task.UpdatedAt = time.Now()
	o.store.SaveTask(ctx, task)

	var finalResponse string
	var lastError error
	maxIterations := 20 // Safety limit

	for iteration := 0; iteration < maxIterations; iteration++ {
		o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Iteration %d: Thinking...", iteration+1), iteration)

		// Call LLM to get next action
		toolCalls, response, err := o.planner.Continue(ctx, conversation, memories)
		if err != nil {
			task.Status = "failed"
			task.Error = fmt.Sprintf("LLM call failed: %w", err)
			task.UpdatedAt = time.Now()
			o.store.SaveTask(ctx, task)
			return task, err
		}

		// Case 1: LLM returned text - we're done
		if response != "" {
			finalResponse = response
			o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Final response: %s", truncate(response, 200)), iteration)
			break
		}

		// Case 2: LLM returned tool calls - execute them
		if len(toolCalls) > 0 {
			// Add assistant message with tool calls to conversation
			assistantMsg := planner.ConversationMessage{
				Role:      "assistant",
				ToolCalls: toolCalls,
			}
			conversation = append(conversation, assistantMsg)

			// Execute each tool call
			for _, toolCall := range toolCalls {
				task.CurrentStep = iteration
				task.UpdatedAt = time.Now()
				o.store.SaveTask(ctx, task)

				toolName := toolCall.Function.Name
				o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Executing: %s", toolName), iteration)

				// Get tool
				tool, ok := o.tools[toolName]
				if !ok {
					lastError = fmt.Errorf("unknown tool: %s", toolName)
					o.logWriter.Write(ctx, task.ID, "error", lastError.Error(), iteration)

					// Add error as tool result
					conversation = append(conversation, planner.ConversationMessage{
						Role:    "tool",
						Content: fmt.Sprintf("Error: %v", lastError),
						ToolID:  toolCall.ID,
					})
					continue
				}

				// Parse arguments
				var args map[string]any
				if toolCall.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						lastError = fmt.Errorf("failed to parse arguments: %w", err)
						o.logWriter.Write(ctx, task.ID, "error", lastError.Error(), iteration)

						conversation = append(conversation, planner.ConversationMessage{
							Role:    "tool",
							Content: fmt.Sprintf("Error: %v", lastError),
							ToolID:  toolCall.ID,
						})
						continue
					}
				}

				// Execute tool with retry logic (Claude Code style)
				result := o.executeWithRetry(ctx, tool, toolName, args, task.ID, iteration)

				// Handle execution errors
				if !result.Success && result.Error == "" {
					lastError = fmt.Errorf("tool execution error")
					o.logWriter.Write(ctx, task.ID, "error", lastError.Error(), iteration)

					conversation = append(conversation, planner.ConversationMessage{
						Role:    "tool",
						Content: fmt.Sprintf("Error: %v", lastError),
						ToolID:  toolCall.ID,
					})
					continue
				}

				// Log result with execution context
				if result.Success {
					o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Output: %s", truncate(result.Output, 500)), iteration)
				} else {
					// Log failure type and retry info
					retryInfo := ""
					if dataMap, ok := result.Data.(map[string]any); ok {
						if hist, ok := dataMap["execution_history"].(types.ExecutionHistory); ok {
							retryInfo = fmt.Sprintf(" (attempts: %d)", len(hist.Attempts))
						}
					}
					o.logWriter.Write(ctx, task.ID, "error", fmt.Sprintf("Tool error [%s]: %s%s", result.FailureType, result.Error, retryInfo), iteration)
					lastError = fmt.Errorf("tool failed: %s", result.Error)
				}

				// Add tool result to conversation with enhanced context
				resultContent := result.Output
				if !result.Success {
					// Add suggestions for alternatives
					if len(result.Alternatives) > 0 {
						resultContent = fmt.Sprintf("Error: %s\n\nAlternatives to try: %v", result.Error, result.Alternatives)
					} else {
						resultContent = fmt.Sprintf("Error: %s", result.Error)
					}
				}
				conversation = append(conversation, planner.ConversationMessage{
					Role:    "tool",
					Content: resultContent,
					ToolID:  toolCall.ID,
				})
			}
		}
	}

	// Check if we hit the iteration limit
	if len(conversation) > maxIterations {
		task.Status = "failed"
		task.Error = "Maximum iterations reached"
		task.UpdatedAt = time.Now()
		o.store.SaveTask(ctx, task)
		return task, fmt.Errorf("maximum iterations (%d) reached", maxIterations)
	}

	// Finalize task
	if lastError == nil {
		task.Status = "completed"
		o.logWriter.Write(ctx, task.ID, "info", "Task completed successfully", len(conversation))
	} else {
		task.Status = "completed" // Still mark as completed if we got a response
		o.logWriter.Write(ctx, task.ID, "warning", fmt.Sprintf("Task completed with some errors: %v", lastError), len(conversation))
	}

	// Store final response if available
	if finalResponse != "" {
		task.Response = finalResponse
	}

	now := time.Now()
	task.CompletedAt = &now
	task.UpdatedAt = now
	o.store.SaveTask(ctx, task)

	// Save conversation to session
	if o.sessionManager != nil && o.sessionID != "" {
		// Save user message
		if err := o.sessionManager.AddMessage(ctx, o.sessionID, "user", message, ""); err == nil {
			o.logWriter.Write(ctx, task.ID, "info", "Saved user message to session", 0)
		}

		// Save assistant response if available
		if finalResponse != "" {
			if err := o.sessionManager.AddMessage(ctx, o.sessionID, "assistant", finalResponse, ""); err == nil {
				o.logWriter.Write(ctx, task.ID, "info", "Saved assistant response to session", 0)
			}
		}
	}

	// Create memory from result
	summary := fmt.Sprintf("Executed: %s", message)
	var resultContent string
	if lastError == nil {
		resultContent = "Successfully completed task"
		if finalResponse != "" {
			resultContent = fmt.Sprintf("Response: %s", truncate(finalResponse, 500))
		}
	} else {
		resultContent = fmt.Sprintf("Completed with errors: %v", lastError)
	}

	mem, err := o.memory.SummarizeResult(ctx, userID, task.ID, summary, resultContent)
	if err == nil && mem != nil {
		mem.ID = uuid.New().String()
		mem.CreatedAt = time.Now()
		mem.UpdatedAt = time.Now()
		o.memory.Store(ctx, mem)
	}

	return task, lastError
}

// ExecuteWithPlan is the old execution method - kept for backward compatibility
func (o *Orchestrator) ExecuteWithPlan(ctx context.Context, userID, message string) (*Task, error) {
	// Create task
	task := &Task{
		ID:        uuid.New().String(),
		UserID:    userID,
		Message:   message,
		Status:    "planning",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save initial task state
	if err := o.store.SaveTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	// Retrieve relevant memories
	memories, err := o.memory.Retrieve(ctx, userID, message)
	if err != nil {
		o.logWriter.Write(ctx, task.ID, "warning", fmt.Sprintf("Failed to retrieve memories: %v", err), 0)
	}

	// Generate plan or get conversational response
	o.logWriter.Write(ctx, task.ID, "info", "Processing request...", 0)
	plan, response, err := o.planner.Plan(ctx, message, memories)
	if err != nil {
		task.Status = "failed"
		task.Error = fmt.Sprintf("Planning failed: %w", err)
		task.UpdatedAt = time.Now()
		o.store.SaveTask(ctx, task)
		return task, err
	}

	// Case 1: Conversational response - no tools needed
	if plan == nil && response != "" {
		task.Response = response
		task.Status = "completed"
		now := time.Now()
		task.CompletedAt = &now
		task.UpdatedAt = now
		o.store.SaveTask(ctx, task)
		o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Response: %s", truncate(response, 200)), 0)
		return task, nil
	}

	// Case 2: Plan with tool execution
	task.Plan = plan
	task.Status = "executing"
	task.UpdatedAt = time.Now()
	o.store.SaveTask(ctx, task)

	o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Plan: %s", plan.Description), 0)

	// Execute plan steps
	var lastError error
	for i, step := range plan.Steps {
		task.CurrentStep = i
		task.UpdatedAt = time.Now()
		o.store.SaveTask(ctx, task)

		o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Step %d: %s", i+1, step.Description), i)

		// Get tool
		tool, ok := o.tools[step.Tool]
		if !ok {
			lastError = fmt.Errorf("unknown tool: %s", step.Tool)
			o.logWriter.Write(ctx, task.ID, "error", lastError.Error(), i)
			continue
		}

		// Execute tool
		result, err := tool.Execute(ctx, step.Args)
		if err != nil {
			lastError = fmt.Errorf("tool execution error: %w", err)
			o.logWriter.Write(ctx, task.ID, "error", lastError.Error(), i)
			task.Status = "failed"
			task.Error = lastError.Error()
			break
		}

		// Log result
		if result.Success {
			o.logWriter.Write(ctx, task.ID, "info", fmt.Sprintf("Output: %s", truncate(result.Output, 500)), i)
		} else {
			o.logWriter.Write(ctx, task.ID, "error", fmt.Sprintf("Tool error: %s", result.Error), i)
			lastError = fmt.Errorf("tool failed: %s", result.Error)
			// Continue to next step unless it's critical
		}
	}

	// Finalize task
	if lastError == nil {
		task.Status = "completed"
		o.logWriter.Write(ctx, task.ID, "info", "Task completed successfully", len(plan.Steps))
	} else {
		task.Status = "failed"
		task.Error = lastError.Error()
	}

	now := time.Now()
	task.CompletedAt = &now
	task.UpdatedAt = now
	o.store.SaveTask(ctx, task)

	// Create memory from result
	summary := fmt.Sprintf("Executed: %s", message)
	var resultContent string
	if lastError == nil {
		resultContent = "Successfully completed all steps"
	} else {
		resultContent = fmt.Sprintf("Completed with errors: %v", lastError)
	}

	mem, err := o.memory.SummarizeResult(ctx, userID, task.ID, summary, resultContent)
	if err == nil && mem != nil {
		mem.ID = uuid.New().String()
		mem.CreatedAt = time.Now()
		mem.UpdatedAt = time.Now()
		o.memory.Store(ctx, mem)
	}

	return task, lastError
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StreamLogWriter implements LogWriter with a callback
type StreamLogWriter struct {
	callback func(ctx context.Context, taskID, level, message string, step int)
}

// NewStreamLogWriter creates a new stream log writer
func NewStreamLogWriter(callback func(ctx context.Context, taskID, level, message string, step int)) *StreamLogWriter {
	return &StreamLogWriter{callback: callback}
}

// Write writes a log entry
func (w *StreamLogWriter) Write(ctx context.Context, taskID, level, message string, step int) error {
	if w.callback != nil {
		w.callback(ctx, taskID, level, message, step)
	}
	return nil
}

// StorageLogWriter combines storage and streaming
type StorageLogWriter struct {
	store    Store
	streamer *StreamLogWriter
}

// NewStorageLogWriter creates a new storage log writer
func NewStorageLogWriter(store Store, streamer *StreamLogWriter) *StorageLogWriter {
	return &StorageLogWriter{
		store:    store,
		streamer: streamer,
	}
}

// Write writes a log entry to both storage and stream
func (w *StorageLogWriter) Write(ctx context.Context, taskID, level, message string, step int) error {
	// Save to storage
	log := &TaskLog{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		Level:     level,
		Message:   message,
		Step:      step,
		Timestamp: time.Now(),
	}
	w.store.SaveTaskLog(ctx, log)

	// Also stream
	if w.streamer != nil {
		w.streamer.Write(ctx, taskID, level, message, step)
	}

	return nil
}
