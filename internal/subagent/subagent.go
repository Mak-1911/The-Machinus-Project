package subagent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/robfig/cron/v3"
)

// Manager manages scheduled subagents
type Manager struct {
	orchestrator *agent.Orchestrator
	store        Store
	cron         *cron.Cron
	enabled      bool
}

// Store defines the storage interface for subagents
type Store interface {
	GetEnabledSubagents(ctx context.Context) ([]Subagent, error)
	UpdateSubagentRunTime(ctx context.Context, id string, lastRun, nextRun time.Time) error
}

// Subagent represents a scheduled agent
type Subagent struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schedule    string                 `json:"schedule"` // cron expression
	Config      map[string]interface{} `json:"config"`
	Enabled     bool                   `json:"enabled"`
	LastRun     *time.Time             `json:"last_run"`
	NextRun     *time.Time             `json:"next_run"`
}

// Job represents a subagent job
type Job struct {
	Subagent  Subagent
	Message   string
	UserID    string
}

// NewManager creates a new subagent manager
func NewManager(orchestrator *agent.Orchestrator, store Store, enabled bool) *Manager {
	return &Manager{
		orchestrator: orchestrator,
		store:        store,
		cron:         cron.New(),
		enabled:      enabled,
	}
}

// Start starts the cron scheduler
func (m *Manager) Start(ctx context.Context) error {
	if !m.enabled {
		log.Println("Subagent system disabled")
		return nil
	}

	log.Println("Starting subagent manager...")

	// Load enabled subagents
	subagents, err := m.store.GetEnabledSubagents(ctx)
	if err != nil {
		return fmt.Errorf("failed to load subagents: %w", err)
	}

	// Register schedules
	for _, sub := range subagents {
		if err := m.RegisterSubagent(ctx, sub); err != nil {
			log.Printf("Failed to register subagent %s: %v", sub.ID, err)
		}
	}

	m.cron.Start()
	log.Printf("Subagent manager started with %d jobs", len(subagents))

	return nil
}

// RegisterSubagent registers a subagent with the cron scheduler
func (m *Manager) RegisterSubagent(ctx context.Context, sub Subagent) error {
	// Extract message from config or use default
	message, ok := sub.Config["message"].(string)
	if !ok {
		message = fmt.Sprintf("Execute scheduled task: %s", sub.Name)
	}

	// Create job
	job := Job{
		Subagent: sub,
		Message:  message,
		UserID:   sub.UserID,
	}

	// Add to cron
	_, err := m.cron.AddFunc(sub.Schedule, func() {
		m.executeJob(ctx, job)
	})

	if err != nil {
		return fmt.Errorf("invalid cron schedule: %w", err)
	}

	log.Printf("Registered subagent '%s' with schedule '%s'", sub.Name, sub.Schedule)
	return nil
}

// executeJob executes a subagent job
func (m *Manager) executeJob(ctx context.Context, job Job) {
	log.Printf("Executing subagent job: %s", job.Subagent.Name)

	// Create a new context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Execute via orchestrator
	task, err := m.orchestrator.Execute(ctx, job.UserID, job.Message)
	if err != nil {
		log.Printf("Subagent job failed: %s - %v", job.Subagent.Name, err)
	} else {
		log.Printf("Subagent job completed: %s - task %s", job.Subagent.Name, task.ID)
	}

	// Update last run time
	now := time.Now()
	if err := m.store.UpdateSubagentRunTime(ctx, job.Subagent.ID, now, now); err != nil {
		log.Printf("Failed to update subagent run time: %v", err)
	}
}

// Stop stops the cron scheduler
func (m *Manager) Stop() {
	if m.enabled {
		log.Println("Stopping subagent manager...")
		m.cron.Stop()
	}
}

// CreateSubagent creates a new subagent
func CreateSubagent(userID, name, description, schedule string, config map[string]interface{}) *Subagent {
	return &Subagent{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Config:      config,
		Enabled:     true,
	}
}

// Common schedules
const (
	// Every minute
	ScheduleEveryMinute = "* * * * *"
	// Every 5 minutes
	ScheduleEvery5Minutes = "*/5 * * * *"
	// Every hour
	ScheduleHourly = "0 * * * *"
	// Every day at midnight
	ScheduleDaily = "0 0 * * *"
	// Every week
	ScheduleWeekly = "0 0 * * 0"
	// Every month
	ScheduleMonthly = "0 0 1 * *"
)
