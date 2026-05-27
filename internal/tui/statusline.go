package tui

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// StatusMode indicates what the agent is currently doing.
type StatusMode int

const (
	ModeIdle StatusMode = iota
	ModeStreaming
	ModeThinking
	ModeExecuting
)

// StatusLine is a minimal single-line status component.
type StatusLine struct {
	spinner         spinner.Model
	mode            StatusMode
	thinkingContent string
}

// NewStatusLine creates a new status line with a default spinner.
func NewStatusLine() *StatusLine {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return &StatusLine{spinner: s}
}

// Init implements tea.Model.
func (s *StatusLine) Init() tea.Cmd {
	return s.spinner.Tick
}

// Update implements tea.Model.
func (s *StatusLine) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

// View implements tea.Model.
func (s *StatusLine) View() tea.View {
	return tea.NewView(s.Render())
}

// Render returns the rendered status line, or an empty string when idle.
func (s *StatusLine) Render() string {
	if s.mode == ModeIdle {
		return ""
	}
	var label string
	switch s.mode {
	case ModeStreaming:
		label = "Responding..."
	case ModeThinking:
		if s.thinkingContent != "" {
			label = "Thinking: " + truncate(s.thinkingContent, 50)
		} else {
			label = "Thinking..."
		}
	case ModeExecuting:
		label = "Executing..."
	}
	return s.spinner.View() + "  " + label
}

// SetMode updates the status mode.
func (s *StatusLine) SetMode(mode StatusMode) {
	s.mode = mode
}

// SetThinking sets the thinking content and switches to thinking mode.
func (s *StatusLine) SetThinking(content string) {
	s.thinkingContent = content
	s.mode = ModeThinking
}

// ClearThinking clears the thinking content.
func (s *StatusLine) ClearThinking() {
	s.thinkingContent = ""
}

// IsIdle returns true when no status should be shown.
func (s *StatusLine) IsIdle() bool {
	return s.mode == ModeIdle
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
