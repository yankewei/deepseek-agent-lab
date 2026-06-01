package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yankewei/ds-coding-agent/internal/skills"
	"github.com/yankewei/ds-coding-agent/internal/tui/slashcmd"
)

// slashMenuModel manages slash command matching, navigation, and rendering.
type slashMenuModel struct {
	index  int
	hidden bool
	value  string
	skills []skills.Skill
}

func (m *slashMenuModel) SetSkills(all []skills.Skill) {
	m.skills = append([]skills.Skill(nil), all...)
}

func (m *slashMenuModel) Sync(value string) {
	if !isSlashMenuValue(value) {
		m.index = 0
		m.hidden = false
		m.value = ""
		return
	}
	if value != m.value {
		m.hidden = false
		m.value = value
	}
	m.clampIndex()
}

func (m *slashMenuModel) Update(msg tea.Msg) (string, bool) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok || !m.Active() {
		return "", false
	}
	// Navigation keys are local widget controls, not user-configurable bindings.
	switch keyMsg.String() {
	case "tab":
		return m.Select()
	case "up":
		m.move(-1)
	case "down":
		m.move(1)
	case "esc":
		m.Close()
	}
	return "", false
}

func (m *slashMenuModel) Active() bool {
	return isSlashMenuValue(m.value) && !m.hidden && len(m.Matches()) > 0
}

func (m *slashMenuModel) HasExactMatch() bool {
	for _, match := range m.Matches() {
		if match.Name == m.value {
			return true
		}
	}
	return false
}

func (m *slashMenuModel) Matches() []slashcmd.Command {
	if !isSlashMenuValue(m.value) {
		return nil
	}
	lower := strings.ToLower(m.value)
	var matches []slashcmd.Command
	for _, cmd := range slashcmd.All() {
		if strings.HasPrefix(strings.ToLower(cmd.Name), lower) {
			matches = append(matches, cmd)
		}
	}
	for _, skill := range m.skills {
		cmd := slashcmd.Command{
			Name:        "skill:" + skill.Name,
			Description: skill.Description,
		}
		if strings.HasPrefix(lower, "skill:") {
			if strings.HasPrefix(strings.ToLower(cmd.Name), lower) {
				matches = append(matches, cmd)
			}
		} else if strings.HasPrefix(lower, "/") {
			query := strings.TrimPrefix(lower, "/")
			if strings.HasPrefix(strings.ToLower(skill.Name), query) {
				matches = append(matches, cmd)
			}
		}
	}
	return matches
}

func (m *slashMenuModel) Select() (string, bool) {
	matches := m.Matches()
	if len(matches) == 0 {
		return "", false
	}
	m.clampIndex()
	selected := matches[m.index].Name
	m.hidden = true
	m.value = selected
	return selected, true
}

func (m *slashMenuModel) Close() {
	if !isSlashMenuValue(m.value) {
		return
	}
	m.hidden = true
}

func (m *slashMenuModel) View() string {
	if !m.Active() {
		return ""
	}
	matches := m.Matches()
	total := len(matches)

	const maxVisible = 8
	startIdx := 0
	if total > maxVisible {
		if m.index >= maxVisible {
			startIdx = m.index - maxVisible + 1
		}
		if startIdx+maxVisible > total {
			startIdx = total - maxVisible
		}
	}
	endIdx := min(startIdx+maxVisible, total)

	lines := make([]string, 0, endIdx-startIdx)
	itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("6")).
		Bold(true)
	ellipsisStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i := startIdx; i < endIdx; i++ {
		cmd := matches[i]
		label := strings.TrimPrefix(cmd.Name, "/")
		line := fmt.Sprintf("  %s  %s", label, cmd.Description)
		if i == m.index {
			line = selectedStyle.Render("> " + label + "  " + cmd.Description)
		} else {
			line = itemStyle.Render(line)
		}
		lines = append(lines, line)
	}
	if startIdx > 0 {
		lines = append([]string{ellipsisStyle.Render("  ...")}, lines...)
	}
	if endIdx < total {
		lines = append(lines, ellipsisStyle.Render("  ..."))
	}
	return lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(lines, "\n"))
}

func (m *slashMenuModel) clampIndex() {
	matches := m.Matches()
	if len(matches) == 0 {
		m.index = 0
		return
	}
	if m.index < 0 {
		m.index = len(matches) - 1
	}
	if m.index >= len(matches) {
		m.index = 0
	}
}

func (m *slashMenuModel) move(delta int) {
	matches := m.Matches()
	if len(matches) == 0 {
		m.index = 0
		return
	}
	m.index = (m.index + delta) % len(matches)
	if m.index < 0 {
		m.index += len(matches)
	}
}

func isSlashMenuValue(value string) bool {
	return strings.HasPrefix(value, "/") || strings.HasPrefix(value, "skill:")
}
