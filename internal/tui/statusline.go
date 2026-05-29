package tui

import (
	"fmt"
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
	modelName       string
	promptTokens    int
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

// Render returns the rendered status line.
func (s *StatusLine) Render() string {
	var parts []string
	if s.modelName != "" {
		parts = append(parts, s.modelName)
	}
	if s.promptTokens > 0 {
		parts = append(parts, s.renderContextUsage())
	}
	text := strings.Join(parts, "  ")
	return lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(text)
}

// RenderActivity returns transient model activity for display above the status line.
func (s *StatusLine) RenderActivity() string {
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
	default:
		return ""
	}

	text := strings.Join([]string{s.spinner.View(), label}, "  ")
	return lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(text)
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

// SetModelName sets the model name displayed in the status line.
func (s *StatusLine) SetModelName(name string) {
	s.modelName = name
}

// SetContextTokens sets the prompt token count for context usage display.
func (s *StatusLine) SetContextTokens(tokens int) {
	s.promptTokens = tokens
}

// IsIdle returns true when no status should be shown.
func (s *StatusLine) IsIdle() bool {
	return s.mode == ModeIdle
}

func (s *StatusLine) renderContextUsage() string {
	if s.promptTokens <= 0 {
		return ""
	}
	window := contextWindowForModel(s.modelName)
	if window <= 0 {
		return "ctx " + formatTokens(s.promptTokens)
	}
	return fmt.Sprintf("ctx %s / %s (%s)", formatTokens(s.promptTokens), formatTokens(window), formatPercent(s.promptTokens, window))
}

func contextWindowForModel(model string) int {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "deepseek-v4-flash", "deepseek-v4-pro", "deepseek-chat", "deepseek-reasoner":
		return 1_000_000
	default:
		return 0
	}
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

func formatPercent(used, total int) string {
	if used <= 0 || total <= 0 {
		return "0.0%"
	}
	percent := float64(used) / float64(total) * 100
	if percent < 0.1 {
		return "<0.1%"
	}
	return fmt.Sprintf("%.1f%%", percent)
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
