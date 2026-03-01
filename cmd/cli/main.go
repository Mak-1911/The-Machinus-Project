package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/config"
	"github.com/machinus/cloud-agent/internal/memory"
	"github.com/machinus/cloud-agent/internal/planner"
	"github.com/machinus/cloud-agent/internal/storage"
	"github.com/machinus/cloud-agent/internal/tools"
	"github.com/machinus/cloud-agent/internal/types"
)

// ANSI color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
	ColorBold  = "\033[1m"
)

// InteractiveCLI handles the interactive mode
type InteractiveCLI struct {
	ctx         context.Context
	cfg         *config.Config
	store       *storage.SQLiteStore
	orch        *agent.Orchestrator
	userID      string
	history     []string // Conversation history for context
}

func main() {
	// Load config
	cfg := config.Load()

	// Check if we're in single-shot mode or interactive mode
	if len(os.Args) >= 2 {
		// Single-shot mode: machinus "message"
		message := os.Args[1]
		runSingleShot(cfg, message)
	} else {
		// Interactive mode: machinus
		runInteractive(cfg)
	}
}

func runSingleShot(cfg *config.Config, message string) {
	// Initialize storage
	fmt.Println("Initializing storage...")
	ctx := context.Background()
	store, err := storage.NewSQLiteStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Run migrations
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create CLI user
	cliUserID := "cli-user"
	if err := store.CreateUser(ctx, cliUserID, "CLI User", "cli@example.com"); err != nil {
		log.Printf("Note: %v", err)
	}

	// Initialize components
	toolMap := initializeTools(cfg)
	p := planner.NewPlanner(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, toolMap)

	var memManager *memory.Manager
	if cfg.EnableMemory {
		memManager = memory.NewManager(store, true, cfg.MaxMemories)
	}

	// Create streaming log writer for real-time output
	logWriter := agent.NewStreamingLogWriter(store, true, false)
	orch := agent.NewOrchestrator(p, toolMap, memManager, store, logWriter)

	// Execute
	fmt.Printf("\n%s▶%s Executing: %s\n\n", ColorBold, ColorReset, message)
	fmt.Println(strings.Repeat("─", 70))

	task, err := orch.Execute(ctx, cliUserID, message)

	fmt.Println(strings.Repeat("─", 70))
	fmt.Println()

	if err != nil {
		fmt.Printf("%s❌ Error:%s %v\n", ColorRed, ColorReset, err)
		os.Exit(1)
	}

	// Show response
	if task.Response != "" {
		fmt.Printf("%s🤖 Response:%s\n%s\n\n", ColorPurple, ColorReset, task.Response)
	}

	// Show summary
	fmt.Printf("%s✅%s Task %s\n", ColorGreen, ColorReset, task.Status)
	fmt.Printf("   Duration: %v\n", task.CompletedAt.Sub(task.CreatedAt).Round(time.Millisecond))
	if task.Error != "" {
		fmt.Printf("%s⚠️  Error:%s %s\n", ColorYellow, ColorReset, task.Error)
	}
}

func runInteractive(cfg *config.Config) {
	// Initialize storage
	fmt.Println("Initializing...")
	ctx := context.Background()
	store, err := storage.NewSQLiteStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Run migrations
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create CLI user
	cliUserID := "cli-user"
	if err := store.CreateUser(ctx, cliUserID, "CLI User", "cli@example.com"); err != nil {
		log.Printf("Note: %v", err)
	}

	// Initialize components
	toolMap := initializeTools(cfg)
	p := planner.NewPlanner(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, toolMap)

	var memManager *memory.Manager
	if cfg.EnableMemory {
		memManager = memory.NewManager(store, true, cfg.MaxMemories)
	}

	// Create streaming log writer for real-time output
	logWriter := agent.NewStreamingLogWriter(store, true, false)
	orch := agent.NewOrchestrator(p, toolMap, memManager, store, logWriter)

	// Create CLI instance
	cli := &InteractiveCLI{
		ctx:      ctx,
		cfg:      cfg,
		store:    store,
		orch:     orch,
		userID:   cliUserID,
		history:  []string{},
	}

	// Start interactive loop
	cli.run()
}

func initializeTools(cfg *config.Config) map[string]types.Tool {
	toolMap := make(map[string]types.Tool)

	// Shell tool
	if cfg.EnableSandbox {
		shellTool := tools.NewShellTool(".", cfg.MaxExecutionTime, true)
		toolMap[shellTool.Name()] = shellTool
	}

	// File tools
	toolMap["read_file"] = tools.NewFileReadTool(10 * 1024 * 1024)
	toolMap["write_file"] = tools.NewFileWriteTool(10 * 1024 * 1024)
	toolMap["edit_file"] = tools.NewFileEditTool(10 * 1024 * 1024)

	// Search tools
	toolMap["glob"] = tools.NewGlobTool(1000)
	toolMap["grep"] = tools.NewGrepTool(1000)

	// Mock tool for testing
	toolMap["echo"] = tools.NewMockTool("echo")

	return toolMap
}

func (cli *InteractiveCLI) run() {
	printWelcome()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("\n%sYou:%s ", ColorCyan, ColorReset)

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		// Handle empty input
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			cli.handleCommand(input)
			continue
		}

		// Add to history
		cli.history = append(cli.history, input)

		// Execute request
		cli.executeRequest(input)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}

	fmt.Printf("\n%sGoodbye! 👋%s\n", ColorGreen, ColorReset)
}

func printWelcome() {
	fmt.Printf("\n%s╔════════════════════════════════════════════════════════════╗%s\n", ColorBold, ColorReset)
	fmt.Printf("%s║%s            %s🤖 Machinus Cloud Agent %s                   %s║%s\n", ColorBold, ColorReset, ColorPurple, ColorReset, ColorBold, ColorReset)
	fmt.Printf("%s╚════════════════════════════════════════════════════════════╝%s\n\n", ColorBold, ColorReset)
	fmt.Printf("%sCommands:%s\n", ColorGray, ColorReset)
	fmt.Printf("  /exit  - Exit the agent\n")
	fmt.Printf("  /clear - Clear conversation history\n")
	fmt.Printf("  /help  - Show this help message\n")
	fmt.Printf("\n%sType your message to start interacting with the agent.%s\n\n", ColorGray, ColorReset)
}

func (cli *InteractiveCLI) handleCommand(input string) {
	cmd := strings.ToLower(input)

	switch cmd {
	case "/exit", "/quit":
		fmt.Printf("\n%sExiting...%s\n", ColorGray, ColorReset)
		os.Exit(0)

	case "/clear":
		cli.history = []string{}
		fmt.Printf("%s✓ Conversation history cleared%s\n", ColorGreen, ColorReset)

	case "/help":
		printWelcome()

	default:
		fmt.Printf("%sUnknown command: %s%s\n", ColorRed, input, ColorReset)
		fmt.Printf("Type /help for available commands\n")
	}
}

func (cli *InteractiveCLI) executeRequest(message string) {
	// Execute with streaming logs
	fmt.Printf("\n%sProcessing...%s\n\n", ColorGray, ColorReset)

	task, err := cli.orch.Execute(cli.ctx, cli.userID, message)

	if err != nil {
		fmt.Printf("%s❌ Error:%s %v\n\n", ColorRed, ColorReset, err)
		return
	}

	// Show tool execution if any
	if task.Plan != nil && len(task.Plan.Steps) > 0 {
		fmt.Printf("\n%s🔧 Tool Execution:%s\n", ColorBlue, ColorReset)
		for _, step := range task.Plan.Steps {
			fmt.Printf("  → %s%s%s\n", ColorYellow, step.Tool, ColorReset)
		}
	}

	// Show response
	if task.Response != "" {
		fmt.Printf("\n%s🤖 Agent:%s\n", ColorPurple, ColorReset)
		// Word-wrap the response
		fmt.Printf("%s\n", formatResponse(task.Response))
	}

	// Show duration
	duration := task.CompletedAt.Sub(task.CreatedAt).Round(time.Millisecond)
	fmt.Printf("\n%s└─ Completed in %v%s\n", ColorGray, duration, ColorReset)
}

func formatResponse(text string) string {
	// Simple formatting - you could enhance this with word wrapping
	// For now, just return as-is with proper newlines
	return strings.TrimSpace(text)
}
