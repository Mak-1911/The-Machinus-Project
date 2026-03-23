package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/config"
	"github.com/machinus/cloud-agent/internal/memory"
	"github.com/machinus/cloud-agent/internal/planner"
	"github.com/machinus/cloud-agent/internal/skills"
	"github.com/machinus/cloud-agent/internal/storage"
	"github.com/machinus/cloud-agent/internal/tools"
	"github.com/machinus/cloud-agent/internal/types"
	uiModel "github.com/machinus/cloud-agent/internal/ui/model"
	"github.com/machinus/cloud-agent/internal/ui/common"
	"github.com/machinus/cloud-agent/internal/ui/app"
)

// ============================================
// APP ADAPTER
// ============================================

// appAdapter wraps the actual app.App interface
type appAdapter struct {
	*app.App
	cfg *config.Config
}

func newAppAdapter(cfg *config.Config) *appAdapter {
	return &appAdapter{
		App: app.New(cfg),
		cfg: cfg,
	}
}

// ============================================
// MESSAGES
// ============================================

type (
	// initializationCompleteMsg is sent when agent initialization is complete
	initializationCompleteMsg struct {
		orch      *agent.Orchestrator
		store     *storage.SQLiteStore
		cfg       *config.Config
		sessionID string
		err       error
	}

	// agentResponseMsg is sent when the agent completes a task
	agentResponseMsg struct {
		task *agent.Task
		err  error
	}

	// agentTickMsg is used to periodically check for agent log messages
	agentTickMsg struct{}
)

// ============================================
// MAIN MODEL
// ============================================

// mainModel wraps the UI model with agent integration
type mainModel struct {
	ui             *uiModel.UI
	ctx            context.Context
	cfg            *config.Config
	store          *storage.SQLiteStore
	orch           *agent.Orchestrator
	sessionManager *agent.SessionManager
	sessionID      string
	userID         string

	ready   bool
	loading bool
}

// ============================================
// INITIALIZATION COMMAND
// ============================================

func initializeAgentCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := config.Load()
		ctx := context.Background()

		store, err := storage.NewSQLiteStore(ctx, cfg.DatabaseURL)
		if err != nil {
			return initializationCompleteMsg{err: fmt.Errorf("storage init failed: %w", err)}
		}

		if err := store.Migrate(ctx); err != nil {
			return initializationCompleteMsg{err: fmt.Errorf("migrations failed: %w", err)}
		}

		cliUserID := "cli-user"
		store.CreateUser(ctx, cliUserID, "CLI User", "cli@example.com")

		toolMap := initializeTools(cfg)

		skillsLoader := skills.NewLoader(".")
		if err := skillsLoader.LoadAll(); err != nil {
			skillsLoader = nil
		}

		p := planner.NewPlanner(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, toolMap, skillsLoader)

		var memManager *memory.Manager
		if cfg.EnableMemory {
			memManager = memory.NewManager(store, true, cfg.MaxMemories)
		}

		sessionMgr := agent.NewSessionManager(store, ".")
		session, _ := sessionMgr.GetOrCreateDefaultSession(ctx, "")
		var sessionID string
		if session != nil {
			sessionID = session.ID
		}

		orch := agent.NewOrchestrator(p, toolMap, memManager, store, nil, sessionMgr, sessionID)

		return initializationCompleteMsg{
			orch:      orch,
			store:     store,
			cfg:       cfg,
			sessionID: sessionID,
			err:       nil,
		}
	}
}

// ============================================
// MODEL INITIALIZATION
// ============================================

func initialModel() mainModel {
	// Load config for UI initialization
	cfg := config.Load()

	// Create app adapter
	appAdapter := newAppAdapter(cfg)

	// Create common UI context
	com := common.DefaultCommon(appAdapter.App)

	// Create the UI model
	ui := uiModel.New(com)

	return mainModel{
		ui:       ui,
		ctx:      context.Background(),
		cfg:      cfg,
		loading:  true,
		userID:   "cli-user",
	}
}

func (m mainModel) Init() tea.Cmd {
	// Initialize the UI first (load commands, history, etc.)
	uiCmd := m.ui.Init()
	return tea.Batch(
		initializeAgentCmd(),
		uiCmd,
	)
}

// ============================================
// MODEL UPDATE
// ============================================

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case initializationCompleteMsg:
		m.loading = false
		if msg.err != nil {
			fmt.Printf("Initialization error: %v\n", msg.err)
			return m, tea.Quit
		}

		m.orch = msg.orch
		m.store = msg.store
		m.cfg = msg.cfg
		m.sessionID = msg.sessionID
		m.ready = true

	case agentResponseMsg:
		m.loading = false
		if msg.err != nil {
			// Handle error - UI will display it
		} else if msg.task.Response != "" {
			// Response will be shown in UI
		}

	case agentTickMsg:
		// Keep the tick going for updates
		cmds = append(cmds, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return agentTickMsg{}
		}))

	case tea.KeyMsg:
		// Handle quit globally
		if msg.String() == "ctrl+c" {
			if m.store != nil {
				m.store.Close()
			}
			return m, tea.Quit
		}
	}

	// Update the UI model
	newUI, uiCmd := m.ui.Update(msg)
	m.ui = newUI.(*uiModel.UI)
	if uiCmd != nil {
		cmds = append(cmds, uiCmd)
	}

	return m, tea.Batch(cmds...)
}

// ============================================
// MODEL VIEW
// ============================================

func (m mainModel) View() tea.View {
	if !m.ready {
		if m.loading {
			return tea.View{
				Content: "\n  ⏳ Initializing Machinus Agent...",
			}
		}
		return tea.View{
			Content: "\n  ❌ Failed to initialize",
		}
	}

	// Return the UI's view directly
	return m.ui.View()
}

// ============================================
// TOOL INITIALIZATION
// ============================================

func initializeTools(cfg *config.Config) map[string]types.Tool {
	toolMap := make(map[string]types.Tool)

	if cfg.EnableSandbox {
		shellTool := tools.NewShellTool(".", cfg.MaxExecutionTime, true)
		toolMap[shellTool.Name()] = shellTool
	}

	toolMap["read_file"] = tools.NewFileReadTool(10 * 1024 * 1024)
	toolMap["write_file"] = tools.NewFileWriteTool(10 * 1024 * 1024)
	toolMap["edit_file"] = tools.NewFileEditTool(10 * 1024 * 1024)
	toolMap["glob"] = tools.NewGlobTool(1000)
	toolMap["grep"] = tools.NewGrepTool(1000)
	toolMap["copy"] = tools.NewCopyTool(1)
	toolMap["move"] = tools.NewMoveTool()
	toolMap["delete"] = tools.NewDeleteTool(false)
	toolMap["list"] = tools.NewListTool()
	toolMap["mkdir"] = tools.NewMakeDirectoryTool()
	toolMap["fileinfo"] = tools.NewFileInfoTool()
	toolMap["http"] = tools.NewHTTPTool(30, 10)
	toolMap["websearch"] = tools.NewWebSearchTool(30, 10)

	pinchtabURL := os.Getenv("PINCHTAB_URL")
	if pinchtabURL == "" {
		pinchtabURL = "http://localhost:9867"
	}
	toolMap["browser"] = tools.NewPinchTabTool(pinchtabURL)

	return toolMap
}

// ============================================
// MAIN ENTRY POINT
// ============================================

func main() {
	p := tea.NewProgram(
		initialModel(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
