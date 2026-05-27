package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View implements tea.Model.
func (m *Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	var sections []string
	sections = append(sections, m.history.View())

	if m.status.mode != statusIdle {
		sep := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("─", m.width))
		sections = append(sections, sep)
		sections = append(sections, m.renderStatus())
	}

	sections = append(sections, m.editor.View())

	base := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.approvalRequest != nil {
		modal := m.renderApprovalModal()
		base = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	}

	v := tea.NewView(base)
	v.AltScreen = true
	return v
}

func (m *Model) renderStatus() string {
	switch m.status.mode {
	case statusThinking:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(truncate(m.status.text, m.width))
	case statusResponse:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(truncate(m.status.text, m.width))
	default:
		return ""
	}
}

func (m *Model) renderApprovalModal() string {
	req := m.approvalRequest.req

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).Render("Approval Required"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Action:"), req.Action))
	if req.Subject != "" {
		lines = append(lines, fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Subject:"), req.Subject))
	}
	if req.RiskLevel != "" {
		lines = append(lines, fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Risk:"), req.RiskLevel))
	}
	if req.PolicyReason != "" {
		lines = append(lines, fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Policy:"), req.PolicyReason))
	}
	if len(req.Details) > 0 {
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Details:"))
		for k, v := range req.Details {
			lines = append(lines, fmt.Sprintf("  %s %s", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(k+":"), v))
		}
	}
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Options:"))
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("y")+" approve once")
	if req.SuggestedPolicyAmendment != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("a")+fmt.Sprintf(" always allow prefix: %s", req.SuggestedPolicyAmendment.Prefix))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("n")+" deny")
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Esc or Ctrl+C denies this request."))

	content := strings.Join(lines, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(1, 2).
		Width(min(m.width-4, 80))

	return boxStyle.Render(content)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
