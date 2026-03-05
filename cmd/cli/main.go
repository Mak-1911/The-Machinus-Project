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
	"github.com/machinus/cloud-agent/internal/skills"
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
	ctx            context.Context
	cfg            *config.Config
	store          *storage.SQLiteStore
	orch           *agent.Orchestrator
	sessionManager *agent.SessionManager
	sessionID      string
	userID         string
	history        []string // Conversation history for context
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
	// Track boot time
	bootStart := time.Now()
	fmt.Printf("%sInitializing Machinus...%s\n", ColorGray, ColorReset)

	ctx := context.Background()

	// Initialize storage with timing
	stepStart := time.Now()
	store, err := storage.NewSQLiteStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	fmt.Printf("  ✓ Storage initialized %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Run migrations
	stepStart = time.Now()
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	fmt.Printf("  ✓ Migrations completed %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Create CLI user
	stepStart = time.Now()
	cliUserID := "cli-user"
	if err := store.CreateUser(ctx, cliUserID, "CLI User", "cli@example.com"); err != nil {
		log.Printf("Note: %v", err)
	}
	fmt.Printf("  ✓ User configured %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Initialize components
	stepStart = time.Now()
	toolMap := initializeTools(cfg)
	fmt.Printf("  ✓ Tools loaded %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	stepStart = time.Now()
	skillsLoader := skills.NewLoader(".")
	if err := skillsLoader.LoadAll(); err != nil {
		log.Printf("Warning: failed to load skills: %v", err)
		// Continue without skills
		skillsLoader = nil
	}
	fmt.Printf("  ✓ Skills loaded %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	stepStart = time.Now()
	p := planner.NewPlanner(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, toolMap, skillsLoader)
	fmt.Printf("  ✓ Planner ready %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	var memManager *memory.Manager
	if cfg.EnableMemory {
		stepStart = time.Now()
		memManager = memory.NewManager(store, true, cfg.MaxMemories)
		fmt.Printf("  ✓ Memory enabled %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)
	}

	// Create session manager
	stepStart = time.Now()
	sessionMgr := agent.NewSessionManager(store)
	session, err := sessionMgr.GetOrCreateDefaultSession(ctx, "")
	if err != nil {
		log.Printf("Warning: failed to create session: %v", err)
	}
	fmt.Printf("  ✓ Session ready %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Create streaming log writer for real-time output
	logWriter := agent.NewStreamingLogWriter(store, true, false)
	var sessionID string
	if session != nil {
		sessionID = session.ID
	}

	stepStart = time.Now()
	orch := agent.NewOrchestrator(p, toolMap, memManager, store, logWriter, sessionMgr, sessionID)
	fmt.Printf("  ✓ Orchestrator ready %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Show total boot time
	bootTime := time.Since(bootStart).Round(time.Millisecond)
	fmt.Printf("\n%s▶%s Booted in %v\n\n", ColorBold, ColorReset, bootTime)

	// Execute
	fmt.Printf("%s▶%s Executing: %s\n\n", ColorBold, ColorReset, message)
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
	// Track boot time
	bootStart := time.Now()
	fmt.Printf("%sInitializing Machinus...%s\n", ColorGray, ColorReset)

	ctx := context.Background()

	// Initialize storage with timing
	stepStart := time.Now()
	store, err := storage.NewSQLiteStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	fmt.Printf("  ✓ Storage initialized %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Run migrations
	stepStart = time.Now()
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	fmt.Printf("  ✓ Migrations completed %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Create CLI user
	stepStart = time.Now()
	cliUserID := "cli-user"
	if err := store.CreateUser(ctx, cliUserID, "CLI User", "cli@example.com"); err != nil {
		log.Printf("Note: %v", err)
	}
	fmt.Printf("  ✓ User configured %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Initialize components
	stepStart = time.Now()
	toolMap := initializeTools(cfg)
	fmt.Printf("  ✓ Tools loaded %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	stepStart = time.Now()
	skillsLoader := skills.NewLoader(".")
	if err := skillsLoader.LoadAll(); err != nil {
		log.Printf("Warning: failed to load skills: %v", err)
		// Continue without skills
		skillsLoader = nil
	}
	fmt.Printf("  ✓ Skills loaded %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	stepStart = time.Now()
	p := planner.NewPlanner(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, toolMap, skillsLoader)
	fmt.Printf("  ✓ Planner ready %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	var memManager *memory.Manager
	if cfg.EnableMemory {
		stepStart = time.Now()
		memManager = memory.NewManager(store, true, cfg.MaxMemories)
		fmt.Printf("  ✓ Memory enabled %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)
	}

	// Create session manager
	stepStart = time.Now()
	sessionMgr := agent.NewSessionManager(store)
	session, err := sessionMgr.GetOrCreateDefaultSession(ctx, "")
	if err != nil {
		log.Printf("Warning: failed to create session: %v", err)
	}
	fmt.Printf("  ✓ Session ready %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Create streaming log writer for real-time output
	logWriter := agent.NewStreamingLogWriter(store, true, false)
	var sessionID string
	if session != nil {
		sessionID = session.ID
	}

	stepStart = time.Now()
	orch := agent.NewOrchestrator(p, toolMap, memManager, store, logWriter, sessionMgr, sessionID)
	fmt.Printf("  ✓ Orchestrator ready %s[%v]%s\n", ColorGray, time.Since(stepStart).Round(time.Millisecond), ColorReset)

	// Show total boot time
	bootTime := time.Since(bootStart).Round(time.Millisecond)
	fmt.Printf("\n%s▶%s Booted in %v\n\n", ColorBold, ColorReset, bootTime)

	// Create CLI instance
	cli := &InteractiveCLI{
		ctx:            ctx,
		cfg:            cfg,
		store:          store,
		orch:           orch,
		sessionManager: sessionMgr,
		sessionID:      sessionID,
		userID:         cliUserID,
		history:        []string{},
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

	// File operations
	toolMap["copy"] = tools.NewCopyTool(1)
	toolMap["move"] = tools.NewMoveTool()
	toolMap["delete"] = tools.NewDeleteTool(false)
	toolMap["list"] = tools.NewListTool()
	toolMap["mkdir"] = tools.NewMakeDirectoryTool()
	toolMap["fileinfo"] = tools.NewFileInfoTool()

	// HTTP tool
	toolMap["http"] = tools.NewHTTPTool(30, 10)

	// Browser tool (PinchTab)
	pinchtabURL := os.Getenv("PINCHTAB_URL")
	if pinchtabURL == "" {
		pinchtabURL = "http://localhost:9867"
	}
	toolMap["browser"] = tools.NewPinchTabTool(pinchtabURL)

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
	fmt.Printf("%s║%s            %s🤖 Machinus Cloud Agent %s                     %s║%s\n", ColorBold, ColorReset, ColorPurple, ColorReset, ColorBold, ColorReset)
	fmt.Printf("%s╚════════════════════════════════════════════════════════════╝%s\n\n", ColorBold, ColorReset)
	fmt.Printf("%sCommands:%s\n", ColorGray, ColorReset)
	fmt.Printf("  /exit      - Exit the agent\n")
	fmt.Printf("  /clear     - Clear current session conversation\n")
	fmt.Printf("  /new       - Start a new session\n")
	fmt.Printf("  /sessions  - List all sessions\n")
	fmt.Printf("  /help      - Show this help message\n")
	fmt.Printf("\n%sType your message to start interacting with the agent.%s\n\n", ColorGray, ColorReset)
}

func (cli *InteractiveCLI) handleCommand(input string) {
	cmd := strings.ToLower(input)

	switch cmd {
	case "/exit", "/quit":
		fmt.Printf("\n%sExiting...%s\n", ColorGray, ColorReset)
		os.Exit(0)

	case "/clear":
		if cli.sessionManager != nil && cli.sessionID != "" {
			if err := cli.sessionManager.ClearSession(cli.ctx, cli.sessionID); err != nil {
				fmt.Printf("%s✗ Failed to clear session: %v%s\n", ColorRed, err, ColorReset)
			} else {
				fmt.Printf("%s✓ Session cleared%s\n", ColorGreen, ColorReset)
			}
		} else {
			cli.history = []string{}
			fmt.Printf("%s✓ Conversation history cleared%s\n", ColorGreen, ColorReset)
		}

	case "/new":
		if cli.sessionManager != nil {
			newSession, err := cli.sessionManager.CreateSession(cli.ctx)
			if err != nil {
				fmt.Printf("%s✗ Failed to create session: %v%s\n", ColorRed, err, ColorReset)
				return
			}
			cli.sessionID = newSession.ID
			fmt.Printf("%s✓ Started new session: %s%s\n", ColorGreen, newSession.ID, ColorReset)
		} else {
			fmt.Printf("%s✗ Session manager not available%s\n", ColorRed, ColorReset)
		}

	case "/sessions":
		cli.listSessions()

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

func (cli *InteractiveCLI) listSessions() {
	if cli.sessionManager == nil {
		fmt.Printf("%s✗ Session manager not available%s\n", ColorRed, ColorReset)
		return
	}

	sessions, err := cli.sessionManager.ListSessions(cli.ctx)
	if err != nil {
		fmt.Printf("%s✗ Failed to list sessions: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	if len(sessions) == 0 {
		fmt.Printf("%sNo sessions found%s\n", ColorGray, ColorReset)
		return
	}

	fmt.Printf("\n%sSessions:%s\n", ColorBold, ColorReset)
	for i, session := range sessions {
		indicator := " "
		if session.ID == cli.sessionID {
			indicator = "→"
		}

		statusColor := ColorGreen
		if session.Status == "closed" {
			statusColor = ColorGray
		} else if session.IsExpired() {
			statusColor = ColorRed
		}

		fmt.Printf("  %s %s[%d]%s %s(%s%s%s) - %d messages\n",
			indicator,
			ColorYellow, i+1, ColorReset,
			session.ID[:8],
			statusColor, session.Status, ColorReset,
			len(session.Messages))
	}
	fmt.Println()
}
