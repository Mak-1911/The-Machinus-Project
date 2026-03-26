package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/machinus/cloud-agent/internal/config"
	"github.com/machinus/cloud-agent/internal/ui/app"
	uiconfig "github.com/machinus/cloud-agent/internal/ui/config"
)

type DaemonConfig struct {
	PidFile string
	LogDir  string
	WorkDir string
	Config  *config.Config
}

type Daemon struct {
	pidManager *PIDManager
	logger     *DaemonLogger
	agent      app.AgentCoordinator
	shutdown   chan struct{}
	running    bool
	mu         sync.Mutex
}

// New: Creates a New Daemon instance
func New(cfg DaemonConfig) (* Daemon, error) {
	// Expand ~ in paths
	pidFile := expandPath(cfg.PidFile)
	logDir := expandPath(cfg.LogDir)

	// Create PID Manager
	pidManager := NewPIDManager(pidFile)

	// Check if already running
	if running, pid := pidManager.IsRunning(); running {
		return nil, fmt.Errorf("daemon already running with PID %d", pid)
	}

	// Create Logger
	logger, err := NewLogger(logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create agent coordinator
	uiCfg := uiconfig.NewUIConfig(cfg.Config)
	agent := app.NewAgentCoordinator(cfg.Config, uiCfg)
	return &Daemon{
		pidManager: pidManager,
		logger:     logger,
		agent:      agent,
		shutdown:   make(chan struct{}),
		running:    false,
	}, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
    if len(path) > 0 && path[0] == '~' {
        home, _ := os.UserHomeDir()
        return filepath.Join(home, path[1:])
    }
    return path
}

// Start: starts a daemon process.
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return fmt.Errorf("Daemon already running")
	}

	// Write PID File
	if err := d.pidManager.Write(os.Getpid()); err != nil {
		d.logger.Error(fmt.Sprintf("Failed to write PID: %v", err))
		return fmt.Errorf("failed to write PID: %w", err)
	}
	d.running = true
	d.logger.Info("Daemon Started")
	return nil
}

// Stop: stops the daemon process
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return fmt.Errorf("daemon not running")
	}

	d.logger.Info("Stopping Daemon....")
	d.running = false

	// Close Logger
	if err := d.logger.Close(); err != nil {
		return fmt.Errorf("failed to close logger: %w", err)
	}

	// Remove PID file
	if err := d.pidManager.Remove(); err != nil {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// Run(): Runs the daemon main loop.
//  -> This blocks until signal is received
func (d *Daemon) Run() {
	d.logger.Info("Daemon running, waiting for tasks....")

	// Setup signal handling
	sigChan := SetupSignals()

	// Setup cleanup function
	cleanup := func() {
		d.logger.Info("Shutting Down...")
		d.Stop()
	}

	// Wait for shutdown signal in the background
	go WaitForShutdown(sigChan, d.shutdown, cleanup)

	// Main Loop : Keep Daemon Alive
	// Later : This will process tasks from queue
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.shutdown:
			d.logger.Info("Shutdown signal received")
			return
		case <-ticker.C:
			d.logger.Info("Heartbeat: Daemon Running....")
		}
	}
}

// IsRunning(): Returns whether the daemon is running.
func (d *Daemon) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

