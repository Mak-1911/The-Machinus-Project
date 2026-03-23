// Package dialog provides the user input dialog for agent interactions.
package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/machinus/cloud-agent/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	// AskUserInputID is the identifier for the user input dialog.
	AskUserInputID = "ask_user_input"
)

type AskUserInputState int

const (
	AskUserInputStatePrompting AskUserInputState = iota
	AskUserInputStateSubmitting
	AskUserInputStateCancelled
)

// UserInputDialog represents a dialog for capturing user input during agent execution.
type UserInputDialog struct {
	com         *common.Common
	width       int
	state       AskUserInputState
	input       textinput.Model
	help        help.Model
	message     string
	placeholder string
	defaultVal  string
	options     []string
	selectedIdx int
	onSubmit    func(string) tea.Msg
	onCancel    func() tea.Msg

	keyMap struct {
		Submit key.Binding
		Cancel key.Binding
		Up     key.Binding
		Down   key.Binding
	}
}

var _ Dialog = (*UserInputDialog)(nil)

// NewUserInputDialog creates a new user input dialog.
// message: The prompt to display to the user
// placeholder: Placeholder text for the input field
// defaultVal: Default value if user just presses enter
// options: Optional list of options for user to select from
func NewUserInputDialog(
	com *common.Common,
	message string,
	placeholder string,
	defaultVal string,
	options []string,
	onSubmit func(string) tea.Msg,
	onCancel func() tea.Msg,
) (*UserInputDialog, tea.Cmd) {
	m := &UserInputDialog{
		com:         com,
		width:       70,
		state:       AskUserInputStatePrompting,
		message:     message,
		placeholder: placeholder,
		defaultVal:  defaultVal,
		options:     options,
		selectedIdx: -1,
		onSubmit:    onSubmit,
		onCancel:    onCancel,
	}

	t := com.Styles

	// Initialize text input
	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	if m.placeholder == "" {
		m.input.Placeholder = "Enter your response..."
	} else {
		m.input.Placeholder = m.placeholder
	}
	m.input.SetStyles(t.TextInput)
	m.input.Focus()
	m.input.SetWidth(max(0, m.width-t.Dialog.View.GetHorizontalFrameSize()-6)) // Account for border (2) and padding (4)

	// Set default value if provided
	if m.defaultVal != "" {
		m.input.SetValue(m.defaultVal)
		m.input.CursorEnd()
	}

	// Initialize help
	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	// Setup key bindings
	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "submit"),
	)
	m.keyMap.Cancel = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "cancel"),
	)
	m.keyMap.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "prev"),
	)
	m.keyMap.Down = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "next"),
	)

	// If options provided, select first by default
	if len(m.options) > 0 {
		m.selectedIdx = 0
		m.input.SetValue(m.options[0])
	}

	return m, nil
}

// ID implements Dialog.
func (m *UserInputDialog) ID() string {
	return AskUserInputID
}

// HandleMsg implements Dialog.
func (m *UserInputDialog) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Cancel):
			m.state = AskUserInputStateCancelled
			if m.onCancel != nil {
				return ActionCmd{func() tea.Msg { return m.onCancel() }}
			}
			return ActionClose{}

		case key.Matches(msg, m.keyMap.Submit):
			if m.state == AskUserInputStatePrompting {
				m.state = AskUserInputStateSubmitting
				input := m.input.Value()
				if m.onSubmit != nil {
					return ActionCmd{func() tea.Msg { return m.onSubmit(input) }}
				}
				return ActionClose{}
			}

		case key.Matches(msg, m.keyMap.Up):
			if len(m.options) > 0 && m.selectedIdx > 0 {
				m.selectedIdx--
				m.input.SetValue(m.options[m.selectedIdx])
				m.input.CursorEnd()
			}

		case key.Matches(msg, m.keyMap.Down):
			if len(m.options) > 0 && m.selectedIdx < len(m.options)-1 {
				m.selectedIdx++
				m.input.SetValue(m.options[m.selectedIdx])
				m.input.CursorEnd()
			}

		default:
			// Pass to text input
			cmd := m.updateInput(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	}

	return nil
}

// updateInput updates the text input model.
func (m *UserInputDialog) updateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// Draw implements Dialog.
func (m *UserInputDialog) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	dialogStyle := t.Dialog.View.Width(m.width)
	promptStyle := t.Dialog.SecondaryText
	helpStyle := t.Dialog.HelpView

	// Build content
	content := strings.Builder{}

	// Render title with gradient
	rc := NewRenderContext(t, m.width)
	rc.Title = m.message
	content.WriteString(rc.RenderTitleOnly())

	// Add options if provided
	if len(m.options) > 0 {
		content.WriteString(promptStyle.Render("Select an option:"))
		content.WriteString("\n")

		for i, opt := range m.options {
			if i == m.selectedIdx {
				content.WriteString(t.Dialog.SelectedItem.Render(fmt.Sprintf("► %s", opt)))
			} else {
				content.WriteString(t.Dialog.NormalItem.Render(fmt.Sprintf("  %s", opt)))
			}
			if i < len(m.options)-1 {
				content.WriteString("\n")
			}
		}
		content.WriteString("\n")
		content.WriteString(promptStyle.Render("Or type your own below:"))
		content.WriteString("\n")
	}

	// Input field with rounded border
	inputBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.com.Styles.BorderColor).
		Background(lipgloss.Black)
	inputContent := m.input.View()
	borderedInput := inputBorderStyle.Width(len(inputContent)).Render(inputContent)
	content.WriteString(borderedInput)
	content.WriteString("\n\n")

	// Help text
	content.WriteString(helpStyle.Render(m.help.View(m)))

	// Wrap in dialog
	dialogContent := dialogStyle.Render(content.String())

	// Center the dialog
	DrawCenterCursor(scr, area, dialogContent, m.Cursor())

	return m.Cursor()
}

// Cursor returns the cursor position for this dialog.
func (m *UserInputDialog) Cursor() *tea.Cursor {
	if m.state == AskUserInputStatePrompting {
		cur := InputCursor(m.com.Styles, m.input.Cursor())
		if cur != nil {
			// Account for the rounded border (1 char on left)
			cur.X += 1
		}
		return cur
	}
	return nil
}

// FullHelp returns the full help view.
func (m *UserInputDialog) FullHelp() [][]key.Binding {
	if len(m.options) > 0 {
		return [][]key.Binding{
			{m.keyMap.Submit, m.keyMap.Cancel},
			{m.keyMap.Up, m.keyMap.Down},
		}
	}
	return [][]key.Binding{
		{m.keyMap.Submit, m.keyMap.Cancel},
	}
}

// ShortHelp returns the short help view.
func (m *UserInputDialog) ShortHelp() []key.Binding {
	bindings := []key.Binding{m.keyMap.Submit, m.keyMap.Cancel}
	if len(m.options) > 0 {
		bindings = append(bindings, m.keyMap.Up, m.keyMap.Down)
	}
	return bindings
}

// GetState returns the current dialog state.
func (m *UserInputDialog) GetState() AskUserInputState {
	return m.state
}

// GetValue returns the current input value.
func (m *UserInputDialog) GetValue() string {
	return m.input.Value()
}

// RenderTitleOnly renders just the title part of the dialog.
func (rc *RenderContext) RenderTitleOnly() string {
	titleStyle := rc.TitleStyle
	dialogStyle := rc.ViewStyle.Width(rc.Width)

	var title string
	if len(rc.TitleInfo) > 0 {
		title = common.DialogTitle(rc.Styles, rc.Title+" "+rc.TitleInfo,
			max(0, rc.Width-dialogStyle.GetHorizontalFrameSize()-
				titleStyle.GetHorizontalFrameSize()), rc.TitleGradientFromColor, rc.TitleGradientToColor)
	} else {
		title = common.DialogTitle(rc.Styles, rc.Title,
			max(0, rc.Width-dialogStyle.GetHorizontalFrameSize()-
				titleStyle.GetHorizontalFrameSize()), rc.TitleGradientFromColor, rc.TitleGradientToColor)
	}

	return titleStyle.Render(title)
}
