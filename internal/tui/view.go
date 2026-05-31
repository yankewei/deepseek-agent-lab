package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View implements tea.Model.
func (m *Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	footer := m.renderFooter()
	messageHeight := m.height - lipgloss.Height(footer)
	if messageHeight < 0 {
		messageHeight = 0
	}

	var sections []string
	if messageHeight > 0 {
		sections = append(sections, fixedHeight(m.messageList.RenderHeight(messageHeight), messageHeight))
	}
	sections = append(sections, footer)

	base := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.form != nil {
		base = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.form.View())
	}

	v := tea.NewView(base)
	v.AltScreen = true
	if m.mouseEnabled {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m *Model) renderFooter() string {
	var footerSections []string
	footerSections = append(footerSections, m.renderStatusLine())
	footerSections = append(footerSections, m.renderEditor())
	if menu := m.renderSlashCommandMenu(); menu != "" {
		footerSections = append(footerSections, menu)
	}
	footerSections = append(footerSections, m.renderHelpBar())

	return lipgloss.JoinVertical(lipgloss.Left, footerSections...)
}

func (m *Model) currentMessageListHeight() int {
	if m.height <= 0 {
		return 0
	}
	footer := m.renderFooter()
	messageHeight := m.height - lipgloss.Height(footer)
	if messageHeight < 0 {
		return 0
	}
	return messageHeight
}

func fixedHeight(content string, height int) string {
	if height <= 0 {
		return ""
	}
	if content == "" {
		return strings.Join(make([]string, height), "\n")
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
