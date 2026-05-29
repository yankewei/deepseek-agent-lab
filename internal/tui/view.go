package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View implements tea.Model.
func (m *Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	var sections []string
	sections = append(sections, m.messageList.Render())

	if !m.statusLine.IsIdle() {
		sections = append(sections, m.statusLine.Render())
	}

	sections = append(sections, m.renderEditor())
	if menu := m.renderSlashCommandMenu(); menu != "" {
		sections = append(sections, menu)
	}
	sections = append(sections, m.renderHelpBar())

	base := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.form != nil {
		base = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.form.View())
	}

	v := tea.NewView(base)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
