package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/machinus/cloud-agent/internal/agent"
	"github.com/machinus/cloud-agent/internal/config"
	"github.com/machinus/cloud-agent/internal/memory"
	"github.com/machinus/cloud-agent/internal/planner"
	"github.com/machinus/cloud-agent/internal/skills"
	"github.com/machinus/cloud-agent/internal/storage"
	"github.com/machinus/cloud-agent/internal/tools"
	"github.com/machinus/cloud-agent/internal/types"
)

// ============================================
// STYLES
// ============================================

var (
	primaryColor     = lipgloss.Color("86BBD8")  // Light blue
	userColor        = lipgloss.Color("A6E3A1")  // Green
	assistantColor   = lipgloss.Color("CDD6F4") // Light gray
	mutedColor       = lipgloss.Color("6C7086")  // Muted gray
	borderColor      = lipgloss.Color("45475A")  // Border gray
	errorColor       = lipgloss.Color("F38BA8")  // Red
	toolColor        = lipgloss.Color("F9E2AF")  // Yellow
	toolExecColor    = lipgloss.Color("89B4FA")  // Blue
	logColor         = lipgloss.Color("FAB387")  // Orange

	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	userMsgStyle     = lipgloss.NewStyle().Foreground(userColor).Bold(true)
	assistantMsgStyle = lipgloss.NewStyle().Foreground(assistantColor).Bold(true)
	assistantContentStyle = lipgloss.NewStyle().Foreground(assistantColor)
	helpStyle         = lipgloss.NewStyle().Foreground(mutedColor)
	borderStyle       = lipgloss.NewStyle().Foreground(borderColor)
	errorStyle        = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
	processingStyle   = lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
	toolExecTitleStyle = lipgloss.NewStyle().Foreground(toolExecColor).Bold(true)
	toolNameStyle     = lipgloss.NewStyle().Foreground(toolColor)
	logStyle          = lipgloss.NewStyle().Foreground(logColor)
)

// ============================================
// CUSTOM MESSAGES
// ============================================

type (
	initializationCompleteMsg struct {
		orch      *agent.Orchestrator
		store     *storage.SQLiteStore
		logWriter *TUILogWriter
		err       error
	}

	agentResponseMsg struct {
		task *agent.Task
		err  error
	}

	statusUpdateMsg struct {
		message string
	}
)

// ============================================
// MODEL
// ============================================

type model struct {
	viewport  viewport.Model
	textinput textinput.Model
	messages []message

	ctx            context.Context
	cfg            *config.Config
	store          *storage.SQLiteStore
	orch           *agent.Orchestrator
	sessionManager *agent.SessionManager
	sessionID      string
	userID         string
	logWriter      *TUILogWriter
	logChan        chan agentLogMsg

	ready   bool
	width   int
	height  int
	loading bool
}

type message struct {
	role    string // "user", "assistant", "system", "error", "log"
	content string
}

// ============================================
// INITIALIZATION COMMAND
// ============================================

func initializeAgentCmd(logChan chan agentLogMsg) tea.Cmd {
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

		sessionMgr := agent.NewSessionManager(store)
		session, _ := sessionMgr.GetOrCreateDefaultSession(ctx, "")
		var sessionID string
		if session != nil {
			sessionID = session.ID
		}

		// Use TUI log writer for real-time updates
		logWriter := NewTUILogWriter(logChan)

		orch := agent.NewOrchestrator(p, toolMap, memManager, store, logWriter, sessionMgr, sessionID)

		return initializationCompleteMsg{
			orch:      orch,
			store:     store,
			logWriter: logWriter,
			err:       nil,
		}
	}
}

func executeAgentCmd(orch *agent.Orchestrator, logWriter *TUILogWriter, userID, message string, sessionID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		logWriter.SetTaskID(userID) // Set task ID for log filtering
		task, err := orch.Execute(ctx, userID, message)
		return agentResponseMsg{task: task, err: err}
	}
}

// ============================================
// INIT
// ============================================

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.Prompt = "❯ "
	ti.CharLimit = -1
	ti.Width = 0

	vp := viewport.New(0, 0)

	logChan := make(chan agentLogMsg, 100)

	return model{
		textinput: ti,
		viewport:  vp,
		messages: []message{
			{
				role:    "system",
				content: "Initializing Machinus Agent...",
			},
		},
		ctx:      context.Background(),
		logChan:  logChan,
		loading:  true,
		userID:   "cli-user",
	}
}

func (m model) Init() tea.Cmd {
	return initializeAgentCmd(m.logChan)
}

// ============================================
// UPDATE
// ============================================

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case initializationCompleteMsg:
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, message{
				role:    "error",
				content: fmt.Sprintf("Failed to initialize: %v", msg.err),
			})
			return m, nil
		}

		m.orch = msg.orch
		m.store = msg.store
		m.logWriter = msg.logWriter
		m.ready = true
		m.messages = append(m.messages, message{
			role:    "system",
			content: "✓ Machinus Agent ready! Type your message to start.",
		})
		m.updateViewport()

		// Start listening for log messages
		return m, waitForLogMsg(m.logChan)

	case agentLogMsg:
		// Display real-time log message
		m.messages = append(m.messages, message{
			role:    "log",
			content: msg.FormatMessage(),
		})
		m.updateViewport()

		// Continue listening for more logs
		return m, waitForLogMsg(m.logChan)

	case agentResponseMsg:
		m.loading = false

		if msg.err != nil {
			m.messages = append(m.messages, message{
				role:    "error",
				content: fmt.Sprintf("Error: %v", msg.err),
			})
		} else {
			// Show final response
			if msg.task.Response != "" {
				m.messages = append(m.messages, message{
					role:    "assistant",
					content: msg.task.Response,
				})
			}
		}

		m.updateViewport()
		m.textinput.Focus()
		return m, nil

	case statusUpdateMsg:
		// No-op, just keep the ticker going
		return m, waitForLogMsg(m.logChan)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.store != nil {
				m.store.Close()
			}
			return m, tea.Quit

		case "enter":
			if m.textinput.Value() != "" && !m.loading && m.ready {
				input := m.textinput.Value()
				m.textinput.Reset()

				m.messages = append(m.messages, message{
					role:    "user",
					content: input,
				})
				m.updateViewport()

				m.loading = true
				m.messages = append(m.messages, message{
					role:    "system",
					content: "⏳ Processing...",
				})
				m.updateViewport()

				return m, executeAgentCmd(m.orch, m.logWriter, m.userID, input, m.sessionID)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 5
		m.textinput.Width = msg.Width - 6

		if !m.ready && !m.loading {
			m.updateViewport()
		}
		return m, nil
	}

	if !m.loading {
		m.textinput, cmd = m.textinput.Update(msg)
	}

	m.viewport, cmd = m.viewport.Update(msg)

	return m, cmd
}

// waitForLogMsg creates a command that waits for log messages
func waitForLogMsg(logChan chan agentLogMsg) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		select {
		case logMsg := <-logChan:
			return logMsg
		default:
			return statusUpdateMsg{} // No-op, keep checking
		}
	})
}

// ============================================
// VIEW
// ============================================

func (m model) View() string {
	if !m.ready && m.loading {
		return "\n  Initializing Machinus Agent..."
	}

	title := titleStyle.Render(" Machinus Agent ")
	title += strings.Repeat("─", m.width-lipgloss.Width(title)-2)
	title += borderStyle.Render("╮")

	messages := m.viewport.View()
	messages = borderStyle.Render("") + messages + borderStyle.Render("")

	inputTop := borderStyle.Render("") + strings.Repeat("─", m.width) + borderStyle.Render("")

	inputText := m.textinput.View()

	bottom := borderStyle.Render("") + strings.Repeat("─", m.width) + borderStyle.Render("")

	helpText := "  Enter: send | Ctrl+C: quit"
	if m.loading {
		helpText = "  ⏳ Processing... | Ctrl+C: quit"
	}
	help := helpStyle.Render(helpText)

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s\n%s\n%s",
		title,
		messages,
		inputTop,
		inputText,
		bottom,
		help,
	)
}

// ============================================
// HELPERS
// ============================================

func (m *model) updateViewport() {
	var content strings.Builder

	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			content.WriteString(userMsgStyle.Render("You: "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")

		case "assistant":
			content.WriteString(assistantMsgStyle.Render("🤖 Agent: "))
			content.WriteString(assistantContentStyle.Render(msg.content))
			content.WriteString("\n\n")

		case "log":
			content.WriteString(logStyle.Render(msg.content))
			content.WriteString("\n")

		case "error":
			content.WriteString(errorStyle.Render("❌ Error: "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")

		case "system":
			content.WriteString(processingStyle.Render(msg.content))
			content.WriteString("\n\n")
		}
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
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

	pinchtabURL := os.Getenv("PINCHTAB_URL")
	if pinchtabURL == "" {
		pinchtabURL = "http://localhost:9867"
	}
	toolMap["browser"] = tools.NewPinchTabTool(pinchtabURL)

	return toolMap
}

// ============================================
// MAIN
// ============================================

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
