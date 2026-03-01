package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/memory"
)

// PostgreSQL implementation of storage
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
}

// Close closes the database connection pool
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// Migrate runs the database schema migration
func (s *PostgresStore) Migrate(ctx context.Context) error {
	schema := `
-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT,
    email TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    path TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    message TEXT NOT NULL,
    plan JSONB,
    status TEXT NOT NULL DEFAULT 'pending',
    current_step INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
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
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_timestamp ON task_logs(timestamp);

-- Memories table with pgvector
CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    summary TEXT NOT NULL,
    content TEXT,
    embedding vector(1536),
    tags TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_embedding ON memories USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Subagents table
CREATE TABLE IF NOT EXISTS subagents (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    schedule TEXT NOT NULL,
    config JSONB,
    enabled BOOLEAN DEFAULT true,
    last_run TIMESTAMP,
    next_run TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subagents_user_id ON subagents(user_id);
CREATE INDEX IF NOT EXISTS idx_subagents_enabled ON subagents(enabled);
CREATE INDEX IF NOT EXISTS idx_subagents_next_run ON subagents(next_run);
`
	_, err := s.pool.Exec(ctx, schema)
	return err
}

// SaveTask saves a task to the database
func (s *PostgresStore) SaveTask(ctx context.Context, task *agent.Task) error {
	planJSON, _ := json.Marshal(task.Plan)

	query := `
		INSERT INTO tasks (id, user_id, project_id, message, plan, status, current_step, created_at, updated_at, completed_at, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE
		SET plan = EXCLUDED.plan,
		    status = EXCLUDED.status,
		    current_step = EXCLUDED.current_step,
		    updated_at = EXCLUDED.updated_at,
		    completed_at = EXCLUDED.completed_at,
		    error = EXCLUDED.error
	`
	_, err := s.pool.Exec(ctx, query,
		task.ID, task.UserID, nil, task.Message, planJSON, task.Status,
		task.CurrentStep, task.CreatedAt, task.UpdatedAt, task.CompletedAt, task.Error,
	)
	return err
}

// GetTask retrieves a task by ID
func (s *PostgresStore) GetTask(ctx context.Context, taskID string) (*agent.Task, error) {
	var task agent.Task
	var planJSON []byte

	query := `SELECT id, user_id, message, plan, status, current_step, created_at, updated_at, completed_at, error FROM tasks WHERE id = $1`
	err := s.pool.QueryRow(ctx, query, taskID).Scan(
		&task.ID, &task.UserID, &task.Message, &planJSON, &task.Status,
		&task.CurrentStep, &task.CreatedAt, &task.UpdatedAt, &task.CompletedAt, &task.Error,
	)
	if err != nil {
		return nil, err
	}

	if len(planJSON) > 0 {
		json.Unmarshal(planJSON, &task.Plan)
	}

	return &task, nil
}

// SaveTaskLog saves a task log entry
func (s *PostgresStore) SaveTaskLog(ctx context.Context, log *agent.TaskLog) error {
	query := `
		INSERT INTO task_logs (id, task_id, level, message, step, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := s.pool.Exec(ctx, query, log.ID, log.TaskID, log.Level, log.Message, log.Step, log.Timestamp)
	return err
}

// GetTaskLogs retrieves logs for a task
func (s *PostgresStore) GetTaskLogs(ctx context.Context, taskID string, limit int) ([]agent.TaskLog, error) {
	query := `SELECT id, task_id, level, message, step, timestamp FROM task_logs WHERE task_id = $1 ORDER BY timestamp ASC LIMIT $2`
	rows, err := s.pool.Query(ctx, query, taskID, limit)
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
func (s *PostgresStore) SaveMemory(ctx context.Context, memory *memory.Memory) error {
	query := `
		INSERT INTO memories (id, user_id, summary, content, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE
		SET summary = EXCLUDED.summary,
		    content = EXCLUDED.content,
		    tags = EXCLUDED.tags,
		    updated_at = EXCLUDED.updated_at
	`
	_, err := s.pool.Exec(ctx, query,
		memory.ID, memory.UserID, memory.Summary, memory.Content,
		memory.Tags, memory.CreatedAt, memory.UpdatedAt,
	)
	return err
}

// GetMemoriesByUserID retrieves memories for a user
func (s *PostgresStore) GetMemoriesByUserID(ctx context.Context, userID string, limit int) ([]memory.Memory, error) {
	query := `SELECT id, user_id, summary, content, tags, created_at, updated_at FROM memories WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
	rows, err := s.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []memory.Memory
	for rows.Next() {
		var mem memory.Memory
		if err := rows.Scan(&mem.ID, &mem.UserID, &mem.Summary, &mem.Content, &mem.Tags, &mem.CreatedAt, &mem.UpdatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// SearchSimilar searches for similar memories using text search
func (s *PostgresStore) SearchSimilar(ctx context.Context, userID string, query string, limit int) ([]memory.Memory, error) {
	// Use simple text search for PostgreSQL
	sqlQuery := `
		SELECT id, user_id, summary, content, tags, created_at, updated_at
		FROM memories
		WHERE user_id = $1 AND (summary ILIKE '%' || $2 || '%' OR content ILIKE '%' || $2 || '%')
		ORDER BY created_at DESC
		LIMIT $3
	`
	rows, err := s.pool.Query(ctx, sqlQuery, userID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []memory.Memory
	for rows.Next() {
		var mem memory.Memory
		if err := rows.Scan(&mem.ID, &mem.UserID, &mem.Summary, &mem.Content, &mem.Tags, &mem.CreatedAt, &mem.UpdatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// CreateUser creates a new user
func (s *PostgresStore) CreateUser(ctx context.Context, id, name, email string) error {
	query := `INSERT INTO users (id, name, email) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`
	_, err := s.pool.Exec(ctx, query, id, name, email)
	return err
}

// GetUser retrieves a user by ID
func (s *PostgresStore) GetUser(ctx context.Context, id string) (name, email string, err error) {
	query := `SELECT name, email FROM users WHERE id = $1`
	err = s.pool.QueryRow(ctx, query, id).Scan(&name, &email)
	return
}

// ListTasks returns tasks for a user with pagination
func (s *PostgresStore) ListTasks(ctx context.Context, userID string, limit, offset int) ([]agent.Task, error) {
	query := `SELECT id, user_id, message, status, created_at FROM tasks WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.pool.Query(ctx, query, userID, limit, offset)
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
func (s *PostgresStore) ListProjects(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	query := `SELECT id, name, description, path, created_at FROM projects WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, query, userID)
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
