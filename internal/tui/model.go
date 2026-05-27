package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/tools"
)

// Model is the Bubble Tea model for the agent TUI.
type Model struct {
	width, height int
	history       viewport.Model
	editor        textarea.Model
	status        statusState

	// Agent config
	client       *openai.Client
	modelName    string
	systemPrompt string
	registry     *tools.Registry
	tracker      *execution.Tracker
	toolDefs     []llm.ToolDefinition

	// Markdown renderer for assistant messages.
	renderer *glamour.TermRenderer

	// Conversation state
	messages         []llm.Message
	isRunning        bool
	assistantBuf     strings.Builder
	reasoningBuf     strings.Builder
	pendingToolCalls []llm.ToolCallDef
	eventStream      <-chan llm.Event
	historyContent   string

	// Optional initial prompt to auto-start on init.
	initialPrompt string

	// Pending approval request displayed as a modal overlay.
	approvalRequest *approvalRequestMsg
}

type statusState struct {
	mode statusMode
	text string
}

type statusMode int

const (
	statusIdle statusMode = iota
	statusThinking
	statusResponse
)

// NewModel creates a new TUI model.
func NewModel(client *openai.Client, modelName, systemPrompt string, registry *tools.Registry, tracker *execution.Tracker, initialPrompt string) *Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.ShowLineNumbers = false
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)

	vp := viewport.New()
	vp.SetWidth(80)
	vp.SetHeight(20)

	var toolDefs []llm.ToolDefinition
	for _, t := range registry.All() {
		toolDefs = append(toolDefs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}

	// Initialize markdown renderer. Default to "dark" style because the
	// light theme has poor contrast on light terminals (grey on white).
	// Users can override via GLAMOUR_STYLE env var.
	var renderer *glamour.TermRenderer
	style := os.Getenv("GLAMOUR_STYLE")
	if style == "" {
		style = "dark"
	}
	if r, err := glamour.NewTermRenderer(glamour.WithStylePath(style)); err == nil {
		renderer = r
	}

	return &Model{
		editor:        ta,
		history:       vp,
		client:        client,
		modelName:     modelName,
		systemPrompt:  systemPrompt,
		registry:      registry,
		tracker:       tracker,
		toolDefs:      toolDefs,
		renderer:      renderer,
		status:        statusState{mode: statusIdle},
		initialPrompt: initialPrompt,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	if m.initialPrompt != "" {
		return func() tea.Msg {
			return userSubmittedMsg{text: m.initialPrompt}
		}
	}
	return nil
}

// submit starts a new agent turn with the given user text.
func (m *Model) submit(text string) (*Model, tea.Cmd) {
	if m.isRunning || text == "" {
		return m, nil
	}
	m.isRunning = true
	if m.systemPrompt != "" && len(m.messages) == 0 {
		m.messages = append(m.messages, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	m.messages = append(m.messages, llm.Message{Role: "user", Content: text})
	m.appendHistory("user", text)
	m.assistantBuf.Reset()
	m.reasoningBuf.Reset()
	m.pendingToolCalls = nil
	m.status = statusState{mode: statusResponse, text: "🚀 运行中..."}
	return m, m.startStreamCmd()
}

// startStreamCmd initiates the LLM stream.
func (m *Model) startStreamCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		events, err := llm.Stream(ctx, m.client, m.modelName, m.messages, m.toolDefs)
		if err != nil {
			return errorMsg{err: err}
		}
		return streamStartedMsg{events: events}
	}
}

// executeToolsCmd runs pending tool calls in parallel.
func (m *Model) executeToolsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results := m.runTools(ctx, m.pendingToolCalls)
		return toolResultsMsg{results: results}
	}
}

func (m *Model) runTools(ctx context.Context, calls []llm.ToolCallDef) []toolResult {
	g, ctx := errgroup.WithContext(ctx)
	results := make([]toolResult, len(calls))

	for i, tc := range calls {
		i, tc := i, tc
		g.Go(func() error {
			tool := m.registry.Get(tc.Name)
			if tool == nil {
				results[i] = toolResult{ID: tc.ID, Content: fmt.Sprintf(`{"error": "unknown tool: %s"}`, tc.Name)}
				return nil
			}

			rec := m.tracker.CreateRecord("tool", tc.Name, "", "")
			m.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusRunning})

			output, err := tool.Execute(ctx, tc.Input)
			if err != nil {
				m.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
				results[i] = toolResult{ID: tc.ID, Content: fmt.Sprintf(`{"error": "%s"}`, err.Error())}
				return nil
			}

			m.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusCompleted})
			outJSON, _ := json.Marshal(output)
			results[i] = toolResult{ID: tc.ID, Content: string(outJSON)}
			return nil
		})
	}

	_ = g.Wait()
	return results
}

func (m *Model) appendHistory(role, text string) {
	switch role {
	case "user":
		prefix := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render("You: ")
		m.historyContent += prefix + text + "\n\n"
	case "event":
		prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("• ")
		m.historyContent += prefix + text + "\n\n"
	case "assistant":
		m.historyContent += text + "\n\n"
	}
	m.history.SetContent(m.historyContent)
	m.history.GotoBottom()
}

func (m *Model) updateAssistantHistory() {
	content := m.historyContent + m.assistantBuf.String()
	m.history.SetContent(content)
	m.history.GotoBottom()
}

func (m *Model) updateLayout() {
	editorHeight := lipgloss.Height(m.editor.View())
	statusHeight := 0
	if m.status.mode != statusIdle {
		statusHeight = 1
	}
	historyHeight := m.height - editorHeight - statusHeight - 1 // separator
	if historyHeight < 5 {
		historyHeight = 5
	}
	m.history.SetWidth(m.width)
	m.history.SetHeight(historyHeight)
	m.editor.SetWidth(m.width)
}

// SetProgram injects the tea.Program reference for async sends.
func (m *Model) SetProgram(p *tea.Program) {
	// No-op for now; prompt uses its own program reference.
}

// SetPrompt rebuilds the tool registry with the given approval prompt.
func (m *Model) SetPrompt(prompt approval.Prompt) {
	m.registry = tools.CreateRegistry(m.tracker, prompt)
	m.toolDefs = nil
	for _, t := range m.registry.All() {
		m.toolDefs = append(m.toolDefs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
}

// renderMarkdown renders Markdown source to a styled terminal string.
// On failure it returns the original source unchanged.
func (m *Model) renderMarkdown(src string) string {
	if m.renderer == nil {
		return src
	}
	out, err := m.renderer.Render(src)
	if err != nil {
		return src
	}
	return strings.TrimSpace(out)
}

type toolResult struct {
	ID      string
	Content string
}
