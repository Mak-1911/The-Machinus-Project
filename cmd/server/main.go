package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/machinus/cloud-agent/internal/api"
	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/config"
	"github.com/machinus/cloud-agent/internal/memory"
	"github.com/machinus/cloud-agent/internal/planner"
	"github.com/machinus/cloud-agent/internal/storage"
	"github.com/machinus/cloud-agent/internal/tools"
	"github.com/machinus/cloud-agent/internal/types"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Log configuration
	log.Printf("Starting Machinus Cloud Agent")
	log.Printf("LLM: %s @ %s", cfg.LLMModel, cfg.LLMBaseURL)
	log.Printf("Database: %s", cfg.DatabaseURL)
	log.Printf("Sandbox: %v", cfg.EnableSandbox)

	// Create context for startup
	ctx := context.Background()

	// Initialize storage
	log.Println("Initializing storage...")
	store, err := storage.NewSQLiteStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Run migrations
	log.Println("Running migrations...")
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create default user
	log.Println("Setting up default user...")
	if err := store.CreateUser(ctx, "default-user", "Default User", "user@example.com"); err != nil {
		log.Printf("Warning: failed to create user: %v", err)
	}

	// Initialize tools
	log.Println("Initializing tools...")
	toolMap := make(map[string]types.Tool)

	// Shell tool
	if cfg.EnableSandbox {
		shellTool := tools.NewShellTool(".", cfg.MaxExecutionTime, true)
		toolMap[shellTool.Name()] = shellTool
		log.Println("  - Shell tool enabled")
	}

	// File Tools
	fileReadTool := tools.NewFileReadTool(10 * 1024 * 1024)
	toolMap[fileReadTool.Name()] = fileReadTool
	log.Println("  - File read tool enabled")

	fileWriteTool := tools.NewFileWriteTool(10 * 1024 * 1024)
	toolMap[fileWriteTool.Name()] = fileWriteTool
	log.Println("  - File write tool enabled")

	fileEditTool := tools.NewFileEditTool(10 * 1024 * 1024)
	toolMap[fileEditTool.Name()] = fileEditTool
	log.Println("  - File edit tool enabled")

	// Search Tools
	globTool := tools.NewGlobTool(1000)	
	toolMap[globTool.Name()] = globTool
	log.Println("  - Glob tool enabled")

	grepTool := tools.NewGrepTool(1000)
	toolMap[grepTool.Name()] = grepTool
	log.Println("  - Grep tool enabled")


	// Add mock tool for testing
	mockTool := tools.NewMockTool("echo")
	toolMap[mockTool.Name()] = mockTool
	log.Println("  - Mock tool enabled")

	// Initialize planner
	log.Println("Initializing planner...")
	p := planner.NewPlanner(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, toolMap)

	// Initialize memory manager
	var memManager *memory.Manager
	if cfg.EnableMemory {
		log.Println("Initializing memory system...")
		memManager = memory.NewManager(store, true, cfg.MaxMemories)
	}

	// Create log writer
	logWriter := agent.NewStorageLogWriter(store, nil)

	// Initialize orchestrator
	log.Println("Initializing orchestrator...")
	orchestrator := agent.NewOrchestrator(p, toolMap, memManager, store, logWriter)

	// Initialize API server
	log.Println("Initializing API server...")
	server := api.NewServer(cfg, orchestrator, store)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Println("✓ Server started successfully")
	log.Printf("✓ API available at http://%s:%d", cfg.Host, cfg.Port)
	log.Printf("✓ WebSocket available at ws://%s:%d/ws", cfg.Host, cfg.Port)
	log.Println()
	log.Println("Example usage:")
	log.Printf("  curl -H 'Authorization: Bearer %s' http://localhost:%d/api/chat -d '{\"message\":\"Create a new Go project\"}'", cfg.AuthToken, cfg.Port)
	log.Println()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}
