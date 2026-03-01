package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/config"
)

// Server handles HTTP and WebSocket connections
type Server struct {
	orchestrator *agent.Orchestrator
	config       *config.Config
	router       *gin.Engine
	clients      map[string]*websocket.Conn
	clientsMutex sync.RWMutex
	upgrader     websocket.Upgrader
	store        agent.Store // Add store for fetching tasks/logs
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, orchestrator *agent.Orchestrator, store agent.Store) *Server {
	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	s := &Server{
		config:       cfg,
		orchestrator: orchestrator,
		router:       router,
		clients:      make(map[string]*websocket.Conn),
		store:        store,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for MVP
			},
		},
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Middleware for authentication
	s.router.Use(s.authMiddleware())

	// REST API endpoints
	api := s.router.Group("/api")
	{
		api.POST("/chat", s.handleChat)
		api.GET("/tasks", s.listTasks)
		api.GET("/tasks/:id", s.getTask)
		api.GET("/tasks/:id/logs", s.getTaskLogs)
		api.GET("/projects", s.listProjects)
	}

	// WebSocket endpoint
	s.router.GET("/ws", s.handleWebSocket)

	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

// authMiddleware validates the auth token
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}

		// Remove "Bearer " prefix if present
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if token != s.config.AuthToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Set("user_id", "default-user") // MVP: single user
		c.Next()
	}
}

// handleChat handles synchronous chat requests
func (s *Server) handleChat(c *gin.Context) {
	var req agent.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")

	// Execute task
	task, err := s.orchestrator.Execute(c.Request.Context(), userID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// handleWebSocket handles WebSocket connections for streaming
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := uuid.New().String()
	s.clientsMutex.Lock()
	s.clients[clientID] = conn
	s.clientsMutex.Unlock()

	defer func() {
		s.clientsMutex.Lock()
		delete(s.clients, clientID)
		s.clientsMutex.Unlock()
		conn.Close()
	}()

	// Send welcome message
	sendJSON(conn, map[string]interface{}{
		"type":    "connected",
		"message": "Connected to Machinus Cloud Agent",
	})

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req agent.ChatRequest
		if err := json.Unmarshal(message, &req); err != nil {
			sendJSON(conn, map[string]interface{}{
				"type":    "error",
				"message": "Invalid request format",
			})
			continue
		}

		// Execute task with streaming
		go s.executeWithStreaming(c.Request.Context(), conn, c.GetString("user_id"), req.Message)
	}
}

// executeWithStreaming executes a task and streams logs
func (s *Server) executeWithStreaming(ctx context.Context, conn *websocket.Conn, userID, message string) {
	// Create log writer that sends to WebSocket
	_ = agent.NewStreamLogWriter(func(ctx context.Context, taskID, level, logMessage string, step int) {
		sendJSON(conn, map[string]interface{}{
			"type":    "log",
			"task_id": taskID,
			"level":   level,
			"message": logMessage,
			"step":    step,
		})
	})

	// Create temporary orchestrator with streaming log writer
	// In production, you'd want a cleaner way to handle this
	// For MVP, we'll execute and stream the logs separately

	taskID := uuid.New().String()

	// Send plan notification
	sendJSON(conn, map[string]interface{}{
		"type":    "plan",
		"task_id": taskID,
		"message": "Starting task execution...",
	})

	// Execute using the main orchestrator
	// The orchestrator will use its configured log writer
	task, err := s.orchestrator.Execute(ctx, userID, message)

	// Send completion
	sendJSON(conn, map[string]interface{}{
		"type":    "complete",
		"task_id": taskID,
		"task":    task,
	})

	if err != nil {
		sendJSON(conn, map[string]interface{}{
			"type":    "error",
			"task_id": taskID,
			"message": err.Error(),
		})
	}
}

// listTasks returns tasks for the user
func (s *Server) listTasks(c *gin.Context) {
	userID := c.GetString("user_id")
	_ = userID // TODO: Filter tasks by user

	// Get limit from query params (default 50)
	limit := 50
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	// For MVP, we'll return tasks from storage
	// You'd need to add a GetTasks method to the store interface
	tasks := []agent.Task{}

	c.JSON(http.StatusOK, tasks)
}

// getTask returns a specific task with its outputs
func (s *Server) getTask(c *gin.Context) {
	taskID := c.Param("id")
	userID := c.GetString("user_id")

	// Fetch task from storage
	task, err := s.store.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	// Verify user owns this task
	if task.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// Fetch logs for this task
	logs, err := s.store.GetTaskLogs(c.Request.Context(), taskID, 1000)
	if err != nil {
		logs = []agent.TaskLog{} // Continue without logs
	}

	// Build step outputs from logs
	stepOutputs := make(map[int]map[string]interface{})
	for _, log := range logs {
		if stepOutputs[log.Step] == nil {
			stepOutputs[log.Step] = make(map[string]interface{})
		}

		// Store output by level
		if log.Level == "info" {
			stepOutputs[log.Step]["output"] = log.Message
		} else if log.Level == "error" {
			stepOutputs[log.Step]["error"] = log.Message
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"task":   task,
		"outputs": stepOutputs,
		"logs":   logs,
	})
}

// getTaskLogs returns logs for a task
func (s *Server) getTaskLogs(c *gin.Context) {
	taskID := c.Param("id")
	userID := c.GetString("user_id")

	// Verify task exists and user owns it
	task, err := s.store.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	if task.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// Get limit from query params (default 100)
	limit := 100
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	// Fetch logs
	logs, err := s.store.GetTaskLogs(c.Request.Context(), taskID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// listProjects returns projects for the user
func (s *Server) listProjects(c *gin.Context) {
	// This would query storage for projects
	c.JSON(http.StatusOK, []map[string]interface{}{})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("Starting server on %s", addr)
	return s.router.Run(addr)
}

// sendJSON sends a JSON message over WebSocket
func sendJSON(conn *websocket.Conn, v interface{}) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}

// Broadcast sends a message to all connected clients
func (s *Server) Broadcast(message interface{}) {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	for _, conn := range s.clients {
		sendJSON(conn, message)
	}
}
