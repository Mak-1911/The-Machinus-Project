package model

import (
	"charm.land/lipgloss/v2"
	"github.com/machinus/cloud-agent/internal/ui/agent"
	"github.com/machinus/cloud-agent/internal/ui/common"
	"github.com/machinus/cloud-agent/internal/ui/config"
)

// selectedLargeModel returns the currently selected large language model from
// the config, if one exists.
func (m *UI) selectedLargeModel() *agent.Model {
	cfg := m.com.Config()
	modelCfg, ok := cfg.Models()[config.SelectedModelTypeLarge]
	if !ok {
		return nil
	}

	return &agent.Model{
		Name:                  modelCfg.Name,
		Provider:             modelCfg.Provider,
		CanReason:            modelCfg.CanReason,
		ReasoningLevels:      modelCfg.ReasoningLevels,
		ReasoningEffort:      modelCfg.ReasoningEffort,
		DefaultReasoningEffort: "",
		Think:                modelCfg.Think,
		ContextWindow:        modelCfg.ContextWindow,
	}
}

// landingView renders the landing page view showing the current working
// directory, model information, and LSP/MCP status in a two-column layout.
func (m *UI) landingView() string {
	t := m.com.Styles
	width := m.layout.main.Dx()
	cwd := common.PrettyPath(t, m.com.Config().WorkingDir(), width)

	parts := []string{
		cwd,
	}

	parts = append(parts, "", m.modelInfo(width))
	infoSection := lipgloss.JoinVertical(lipgloss.Left, parts...)

	_ = infoSection

	mcpLspSectionWidth := min(30, (width-1)/2)

	lspSection := m.lspInfo(mcpLspSectionWidth, 10, false)
	mcpSection := m.mcpInfo(mcpLspSectionWidth, 10, false)

	content := lipgloss.JoinHorizontal(lipgloss.Left, lspSection, " ", mcpSection)

	return lipgloss.NewStyle().
		Width(width).
		Height(m.layout.main.Dy() - 1).
		PaddingTop(1).
		Render(
			lipgloss.JoinVertical(lipgloss.Left, infoSection, "", content),
		)
}
