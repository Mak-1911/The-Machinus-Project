package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/memory"
)

// SQLiteStore implements storage using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(ctx context.Context, dbPath string) (*SQLiteStore, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set SQLite pragmas for better performance
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Migrate runs the database schema migration
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	schema := `
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT,
    email TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    message TEXT NOT NULL,
    plan TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    current_step INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    error TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

-- Task logs table
CREATE TABLE IF NOT EXISTS task_logs (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    step INTEGER NOT NULL DEFAULT 0,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_timestamp ON task_logs(timestamp);

-- Memories table (simple version without FTS5)
CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    summary TEXT NOT NULL,
    content TEXT,
    tags TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_summary ON memories(summary);

-- Subagents table
CREATE TABLE IF NOT EXISTS subagents (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    schedule TEXT NOT NULL,
    config TEXT,
    enabled INTEGER DEFAULT 1,
    last_run DATETIME,
    next_run DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subagents_user_id ON subagents(user_id);
CREATE INDEX IF NOT EXISTS idx_subagents_enabled ON subagents(enabled);
CREATE INDEX IF NOT EXISTS idx_subagents_next_run ON subagents(next_run);
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// SaveTask saves a task to the database
func (s *SQLiteStore) SaveTask(ctx context.Context, task *agent.Task) error {
	planJSON, _ := json.Marshal(task.Plan)
	planStr := string(planJSON)

	query := `
		INSERT INTO tasks (id, user_id, message, plan, status, current_step, created_at, updated_at, completed_at, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
		    plan = excluded.plan,
		    status = excluded.status,
		    current_step = excluded.current_step,
		    updated_at = excluded.updated_at,
		    completed_at = excluded.completed_at,
		    error = excluded.error
	`
	_, err := s.db.ExecContext(ctx, query,
		task.ID, task.UserID, task.Message, planStr, task.Status,
		task.CurrentStep, task.CreatedAt, task.UpdatedAt, task.CompletedAt, task.Error,
	)
	return err
}

// GetTask retrieves a task by ID
func (s *SQLiteStore) GetTask(ctx context.Context, taskID string) (*agent.Task, error) {
	var task agent.Task
	var planStr string

	query := `SELECT id, user_id, message, plan, status, current_step, created_at, updated_at, completed_at, error FROM tasks WHERE id = ?`
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&task.ID, &task.UserID, &task.Message, &planStr, &task.Status,
		&task.CurrentStep, &task.CreatedAt, &task.UpdatedAt, &task.CompletedAt, &task.Error,
	)
	if err != nil {
		return nil, err
	}

	if planStr != "" {
		json.Unmarshal([]byte(planStr), &task.Plan)
	}

	return &task, nil
}

// SaveTaskLog saves a task log entry
func (s *SQLiteStore) SaveTaskLog(ctx context.Context, log *agent.TaskLog) error {
	query := `
		INSERT INTO task_logs (id, task_id, level, message, step, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query, log.ID, log.TaskID, log.Level, log.Message, log.Step, log.Timestamp)
	return err
}

// GetTaskLogs retrieves logs for a task
func (s *SQLiteStore) GetTaskLogs(ctx context.Context, taskID string, limit int) ([]agent.TaskLog, error) {
	query := `SELECT id, task_id, level, message, step, timestamp FROM task_logs WHERE task_id = ? ORDER BY timestamp ASC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []agent.TaskLog
	for rows.Next() {
		var log agent.TaskLog
		if err := rows.Scan(&log.ID, &log.TaskID, &log.Level, &log.Message, &log.Step, &log.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// SaveMemory saves a memory
func (s *SQLiteStore) SaveMemory(ctx context.Context, mem *memory.Memory) error {
	// Convert tags to JSON string
	tagsJSON, _ := json.Marshal(mem.Tags)
	tagsStr := string(tagsJSON)

	query := `
		INSERT INTO memories (id, user_id, summary, content, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
		    summary = excluded.summary,
		    content = excluded.content,
		    tags = excluded.tags,
		    updated_at = excluded.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		mem.ID, mem.UserID, mem.Summary, mem.Content,
		tagsStr, mem.CreatedAt, mem.UpdatedAt,
	)
	return err
}

// GetMemoriesByUserID retrieves memories for a user
func (s *SQLiteStore) GetMemoriesByUserID(ctx context.Context, userID string, limit int) ([]memory.Memory, error) {
	query := `SELECT id, user_id, summary, content, tags, created_at, updated_at FROM memories WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []memory.Memory
	for rows.Next() {
		var mem memory.Memory
		var tagsStr string
		if err := rows.Scan(&mem.ID, &mem.UserID, &mem.Summary, &mem.Content, &tagsStr, &mem.CreatedAt, &mem.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsStr), &mem.Tags)
		memories = append(memories, mem)
	}

	return memories, nil
}

// SearchSimilar searches for similar memories using text search
func (s *SQLiteStore) SearchSimilar(ctx context.Context, userID string, query string, limit int) ([]memory.Memory, error) {
	// Use simple LIKE search for compatibility
	sqlQuery := `
		SELECT id, user_id, summary, content, tags, created_at, updated_at
		FROM memories
		WHERE user_id = ? AND (summary LIKE ? OR content LIKE ?)
		ORDER BY created_at DESC
		LIMIT ?
	`
	searchPattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, sqlQuery, userID, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []memory.Memory
	for rows.Next() {
		var mem memory.Memory
		var tagsStr string
		if err := rows.Scan(&mem.ID, &mem.UserID, &mem.Summary, &mem.Content, &tagsStr, &mem.CreatedAt, &mem.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tagsStr), &mem.Tags)
		memories = append(memories, mem)
	}

	return memories, nil
}

// CreateUser creates a new user
func (s *SQLiteStore) CreateUser(ctx context.Context, id, name, email string) error {
	query := `INSERT OR IGNORE INTO users (id, name, email) VALUES (?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, id, name, email)
	return err
}

// GetUser retrieves a user by ID
func (s *SQLiteStore) GetUser(ctx context.Context, id string) (name, email string, err error) {
	query := `SELECT name, email FROM users WHERE id = ?`
	err = s.db.QueryRowContext(ctx, query, id).Scan(&name, &email)
	return
}

// ListTasks returns tasks for a user with pagination
func (s *SQLiteStore) ListTasks(ctx context.Context, userID string, limit, offset int) ([]agent.Task, error) {
	query := `SELECT id, user_id, message, status, created_at FROM tasks WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []agent.Task
	for rows.Next() {
		var task agent.Task
		if err := rows.Scan(&task.ID, &task.UserID, &task.Message, &task.Status, &task.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ListProjects returns projects for a user
func (s *SQLiteStore) ListProjects(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	query := `SELECT id, name, description, path, created_at FROM projects WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []map[string]interface{}
	for rows.Next() {
		var id, name, description, path string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &description, &path, &createdAt); err != nil {
			return nil, err
		}
		projects = append(projects, map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": description,
			"path":        path,
			"created_at":  createdAt,
		})
	}

	return projects, nil
}
