package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/machinus/cloud-agent/internal/ui/config"
	"github.com/machinus/cloud-agent/internal/ui/csync"
	"github.com/machinus/cloud-agent/internal/ui/fsext"
	"github.com/machinus/cloud-agent/internal/ui/lsp"
	"github.com/machinus/cloud-agent/internal/ui/session"
	"github.com/machinus/cloud-agent/internal/ui/common"
	"github.com/machinus/cloud-agent/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const (
	headerDiag           = "╱"
	minHeaderDiags       = 3
	leftPadding          = 1
	rightPadding         = 1
	diagToDetailsSpacing = 1 // space between diagonal pattern and details section
)

type header struct {
	// cached logo and compact logo
	logo        string
	compactLogo string

	com     *common.Common
	width   int
	compact bool
}

// newHeader creates a new header model.
func newHeader(com *common.Common) *header {
	h := &header{
		com: com,
	}
	t := com.Styles
	h.compactLogo = t.Header.Charm.Render("OxaLabs™") + " " +
		styles.ApplyBoldForegroundGrad(t, "CRUSH", t.Secondary, t.Primary) + " "
	return h
}

// drawHeader draws the header for the given session.
func (h *header) drawHeader(
	scr uv.Screen,
	area uv.Rectangle,
	session *session.Session,
	compact bool,
	detailsOpen bool,
	width int,
) {
	t := h.com.Styles
	if width != h.width || compact != h.compact {
		h.logo = renderLogo(h.com.Styles, compact, width)
	}

	h.width = width
	h.compact = compact

	// If no session or app, just show the logo
	if session == nil || h.com.App == nil {
		uv.NewStyledString(h.logo).Draw(scr, area)
		return
	}

	if session.ID == "" {
		uv.NewStyledString(h.logo).Draw(scr, area)
		return
	}

	var b strings.Builder
	b.WriteString(h.compactLogo)

	availDetailWidth := width - leftPadding - rightPadding - lipgloss.Width(b.String()) - minHeaderDiags - diagToDetailsSpacing
	details := renderHeaderDetails(
		h.com,
		session,
		nil,
		detailsOpen,
		availDetailWidth,
		compact,
	)

	remainingWidth := width -
		lipgloss.Width(b.String()) -
		lipgloss.Width(details) -
		leftPadding -
		rightPadding -
		diagToDetailsSpacing

	if remainingWidth > 0 {
		b.WriteString(t.Header.Diagonals.Render(
			strings.Repeat(headerDiag, max(minHeaderDiags, remainingWidth)),
		))
		b.WriteString(" ")
	}

	b.WriteString(details)

	view := uv.NewStyledString(
		t.Base.Padding(0, rightPadding, 0, leftPadding).Render(b.String()))
	view.Draw(scr, area)
}

// renderHeaderDetails renders the details section of the header.
func renderHeaderDetails(
	com *common.Common,
	session *session.Session,
	lspClients *csync.Map[string, *lsp.Client],
	detailsOpen bool,
	availWidth int,
	compact bool,
) string {
	t := com.Styles

	var parts []string

	errorCount := 0
	if lspClients != nil {
		clients := lspClients.Seq()
		for _, l := range clients {
			errorCount += l.GetDiagnosticCounts().Error
		}
	}

	if errorCount > 0 {
		parts = append(parts, t.LSP.ErrorDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPErrorIcon, errorCount)))
	}

	// Add context usage percentage if session and model are available
	if session != nil {
		agentCfg := com.Config().Agents()[config.AgentCoder]
		model := com.Config().GetModelByType(agentCfg.Model)
		if model != nil && model.ContextWindow > 0 {
			percentage := (float64(session.CompletionTokens()+session.PromptTokens()) / float64(model.ContextWindow)) * 100
			formattedPercentage := t.Header.Percentage.Render(fmt.Sprintf("%d%%", int(percentage)))
			parts = append(parts, formattedPercentage)
		}
	}

	// Only show ctrl+d hint in compact mode (when sidebar is hidden)
	if compact {
		const keystroke = "ctrl+d"
		if detailsOpen {
			parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" close"))
		} else {
			parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" open "))
		}
	}

	dot := t.Header.Separator.Render(" • ")
	metadata := strings.Join(parts, dot)
	metadata = dot + metadata

	const dirTrimLimit = 4
	cfg := com.Config()
	cwd := fsext.DirTrim(fsext.PrettyPath(cfg.WorkingDir()), dirTrimLimit)
	cwd = t.Header.WorkingDir.Render(cwd)

	result := cwd + metadata
	return ansi.Truncate(result, max(0, availWidth), "…")
}
