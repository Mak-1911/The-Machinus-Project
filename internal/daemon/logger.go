package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DaemonLogger struct {
	mu            sync.Mutex
	infoFile 	  *os.File
	errorFile  	  *os.File
}

// NewLogger: Creates a new Daemon Logger
//  - Creates ~/.machinus/logs/ directory
//  - Opens agent.log for info messages
//  - Opens error.log for errors
//  - Both in append mode (keeps history)

func NewLogger(logDir string) (*DaemonLogger, error) {
	// Create Log Directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open info log file (append mode)
	infoPath := filepath.Join(logDir, "agent.log")
	infoFile, err := os.OpenFile(infoPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open info log: %w", err)
	}
	// Open error log file (append mode)
	errorPath := filepath.Join(logDir, "error.log")
	errorFile, err := os.OpenFile(errorPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open error log: %w", err)
	}
	return &DaemonLogger{
		infoFile: infoFile,
		errorFile: errorFile,
	}, nil
}

// Info: Logs an informational message
//  - Thread-safe write
//  - Adds timestamp
//  - Format: 2025-01-15 09:30:00 [INFO] Starting daemon...

func (l *DaemonLogger) Info(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("%s [INFO] %s \n", timestamp, msg)
	l.infoFile.WriteString(logLine) 
}

// Error: Logs an error message
//  - Writes to both agent.log and error.log
//  - Errors go to both files for visibility
func (l *DaemonLogger) Error(msg string) {
    l.mu.Lock()
    defer l.mu.Unlock()

    timestamp := time.Now().Format("2006-01-02 15:04:05")
    logLine := fmt.Sprintf("%s [ERROR] %s\n", timestamp, msg)

    // Write to both files
    l.infoFile.WriteString(logLine)
    l.errorFile.WriteString(logLine)
}

// Close: Closes all Log files
// - Closes both files safely
// - Returns any errors
func (l *DaemonLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error
	if err := l.infoFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := l.errorFile.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing logs: %v", errs)
	}
	return nil
}