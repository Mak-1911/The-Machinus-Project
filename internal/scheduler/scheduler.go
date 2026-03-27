package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

    "gopkg.in/yaml.v3"
)

type Scheduler struct {
	tasks 	map[string]*ScheduledTask
	mu    	sync.RWMutex
	ticker	*time.Ticker
	quit 	chan struct{}
	agent 	AgentRunner
	file 	string
}

type AgentRunner interface {
	RunTask(task string) error
}

type Config struct {
	ScheduleFile 	string
	Agent 			AgentRunner
}

// New(): Creates a new Scheduler.
func New(cfg Config) *Scheduler {
	return &Scheduler{
		tasks:  make(map[string]*ScheduledTask),
		ticker: time.NewTicker(1 * time.Second),
		quit: 	make(chan struct{}),
		agent: 	cfg.Agent,
		file: 	cfg.ScheduleFile,
	}
}

// Start(): Starts the scheduler
func (s *Scheduler) Start() error {
	// Load tasks from file
	if err := s.Load(); err != nil {
		return fmt.Errorf("failed to load schedule: %w", err)
	}
	// Start ticker loop
	go s.run()
	return nil
}

// run(): Main Scheduler Loop
func (s *Scheduler) run() {
	for {
		select {
		case <-s.ticker.C:
			s.checkTasks()
		case <-s.quit:
			return
		}
	}
}


// checkTasks(): checks if any tasks are due to run
func (s *Scheduler) checkTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, task := range s.tasks {
		if !task.Enabled {
			continue
		}
		if now.After(task.NextRun) || now.Equal(task.NextRun) {
			// if task due, run it
			go s.runTask(task)
			s.UpdateNextRun(task)
		}
	}
}

// runTask(): Executes a scheduled task
func (s *Scheduler) runTask(task *ScheduledTask) {
	task.LastRun = time.Now()
	// Run task via agent
	if err := s.agent.RunTask(task.Task); err != nil {
		fmt.Printf("Error running task '%s': %v\n", task.Name, err)
		return
	}
	fmt.Printf("Task '%s' completed\n", task.Name)
}

// UpdateNextRun(): Calculates and updates the next run time for a task/
func (s *Scheduler) UpdateNextRun(task *ScheduledTask) {
	cronExpression, err := Parse(task.Cron)
	if err != nil {
		fmt.Printf("Error parsing cron for task '%s': %v\n", task.Name, err)
		return
	}
	task.NextRun = cronExpression.GetNextRun(time.Now())
}

// Add(): adds a new scheduled task
func(s *Scheduler) Add(task *ScheduledTask) error {
	// Validate cron
	if !IsValid(task.Cron) {
		return fmt.Errorf("Invalid cron expression: %s", task.Cron)
	}
	// Calculate next run time
	cronExpression, err := Parse(task.Cron)
	if err != nil {
		return fmt.Errorf("failed to parse cron: %w", err )
	}
	task.NextRun = cronExpression.GetNextRun(time.Now())
	task.CreatedAt = time.Now()

	if task.ID == "" {
		task.ID = generateID()
	}

	// Lock to add to map
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	// Save to file (without holding lock)
	if err := s.Save(); err != nil {
		return fmt.Errorf("failed to save Schedule: %w", err)
	}
	return nil
}

// Remove(): removes a schedules task.
func (s *Scheduler) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return fmt.Errorf("Task not found: %s", id)
	}
	delete(s.tasks, id)

	// Unlock before saving
	s.mu.Unlock()
	defer s.mu.Lock()  // Re-lock for defer

	return s.Save()
}

// Get(): retrieves a task by ID
func (s *Scheduler) Get(id string) (*ScheduledTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

// List(): Returns all scheduled tasks.
func (s *Scheduler) List() []*ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// Enable(): enables a task.
func (s *Scheduler) Enable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}
	task.Enabled = true

	// Unlock before saving
	s.mu.Unlock()
	defer s.mu.Lock()

	return s.Save()
}

// Disable(): disables a task
func (s *Scheduler) Disable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}
	task.Enabled = false

	// Unlock before saving
	s.mu.Unlock()
	defer s.mu.Lock()

	return s.Save()
}

// Load(): loads scheduled tasks from file
func (s *Scheduler) Load() error {
	// Expand ~ in path
	file := expandPath(s.file)
	// If file does not exist, create empty
	if _, err := os.Stat(file); os.IsNotExist(err) {
		// Create Directory
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			return fmt.Errorf("Failed to create directory: %w", err)
		}
		// Create empty file
		cfg := &ScheduleConfig{Schedule: []*ScheduledTask{}}
		return saveConfig(file, cfg)
	}
	// Load file
	cfg, err := loadConfig(file)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks = make(map[string]*ScheduledTask)
	for _, task := range cfg.Schedule {
		s.tasks[task.ID] = task
	}
	return nil
}

//  Save(): saves scheduled tasks to file.
// Note: Caller must hold appropriate lock.
func (s *Scheduler) Save() error {
	// Don't lock here - caller should already hold lock
	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	cfg := &ScheduleConfig{Schedule: tasks}
	return saveConfig(expandPath(s.file), cfg)
}

// Stop: stops the scheduler
func (s *Scheduler) Stop() {
	s.ticker.Stop()
	close(s.quit)
}

// expandPath: expands ~ to home directory
func expandPath(path string) string {
    if len(path) > 0 && path[0] == '~' {  // ✅ Correct (compare byte to byte)
        home, _ := os.UserHomeDir()
        return filepath.Join(home, path[1:])
    }
    return path
}

// generateID generates a unique task ID.
func generateID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

// loadConfig loads schedule configuration from YAML file.
func loadConfig(path string) (*ScheduleConfig, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)    
    }

    var cfg ScheduleConfig
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse YAML: %w", err)   
    }

    return &cfg, nil
}

// saveConfig saves schedule configuration to YAML file.
func saveConfig(path string, cfg *ScheduleConfig) error {
    data, err := yaml.Marshal(cfg)
    if err != nil {
        return fmt.Errorf("failed to marshal YAML: %w", err)      
	}

    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("failed to write file: %w", err)        
    }

    return nil
}