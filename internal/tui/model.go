package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
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

	messageList *MessageList
	editor      textinput.Model
	statusLine  *StatusLine
	keys        KeyMap

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
	eventStream      <-chan llm.Event
	cancelStream     context.CancelFunc
	thinkingBuf      string
	pendingToolCalls []llm.ToolCallDef
	toolCallInputs   map[string]string // toolID -> argsJSON

	// Optional initial prompt to auto-start on init.
	initialPrompt string

	// Pending approval state.
	approvalReq   *approval.Request
	approvalResCh chan approval.Result
	form          *huh.Form
}

// NewModel creates a new TUI model.
func NewModel(client *openai.Client, modelName, systemPrompt string, registry *tools.Registry, tracker *execution.Tracker, initialPrompt string) *Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = ""
	ti.Focus()
	ti.SetWidth(80)

	// Style the input so text is clearly visible.
	styles := textinput.DefaultStyles(true)
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	ti.SetStyles(styles)

	var toolDefs []llm.ToolDefinition
	for _, t := range registry.All() {
		toolDefs = append(toolDefs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}

	var renderer *glamour.TermRenderer
	style := os.Getenv("GLAMOUR_STYLE")
	if style == "" {
		style = "notty"
	}
	if r, err := glamour.NewTermRenderer(glamour.WithStylePath(style)); err == nil {
		renderer = r
	}

	return &Model{
		editor:         ti,
		messageList:    NewMessageList(),
		statusLine:     NewStatusLine(),
		keys:           DefaultKeyMap(),
		client:         client,
		modelName:      modelName,
		systemPrompt:   systemPrompt,
		registry:       registry,
		tracker:        tracker,
		toolDefs:       toolDefs,
		renderer:       renderer,
		toolCallInputs: make(map[string]string),
		initialPrompt:  initialPrompt,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.RequestBackgroundColor,
		m.statusLine.Init(),
	}
	if m.initialPrompt != "" {
		cmds = append(cmds, func() tea.Msg {
			return userSubmittedMsg{text: m.initialPrompt}
		})
	}
	return tea.Batch(cmds...)
}

// submit starts a new agent turn with the given user text.
func (m *Model) submit(text string) tea.Cmd {
	if m.isRunning || text == "" {
		return nil
	}
	m.isRunning = true
	if m.systemPrompt != "" && len(m.messages) == 0 {
		m.messages = append(m.messages, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	m.messages = append(m.messages, llm.Message{Role: "user", Content: text})
	m.messageList.Add(Message{Type: MsgUser, Content: text, Status: StatusDone})
	m.statusLine.SetMode(ModeStreaming)
	m.editor.Reset()
	return m.startStreamCmd()
}

// startStreamCmd initiates the LLM stream with a cancellable context.
func (m *Model) startStreamCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		events, err := llm.Stream(ctx, m.client, m.modelName, m.messages, m.toolDefs)
		if err != nil {
			cancel()
			return errorMsg{err: err}
		}
		return streamStartedMsg{events: events, cancel: cancel}
	}
}

// executeToolsCmd runs the given tool calls in parallel.
func (m *Model) executeToolsCmd(calls []llm.ToolCallDef) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results := m.runTools(ctx, calls)
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
				results[i] = toolResult{ID: tc.ID, Name: tc.Name, Content: fmt.Sprintf(`{"error": "unknown tool: %s"}`, tc.Name)}
				return nil
			}

			rec := m.tracker.CreateRecord("tool", tc.Name, "", "")
			m.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusRunning})

			output, err := tool.Execute(ctx, tc.Input)
			if err != nil {
				m.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
				results[i] = toolResult{ID: tc.ID, Name: tc.Name, Content: fmt.Sprintf(`{"error": "%s"}`, err.Error()), Err: err}
				return nil
			}

			m.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusCompleted})
			outJSON, _ := json.Marshal(output)
			results[i] = toolResult{ID: tc.ID, Name: tc.Name, Content: string(outJSON)}
			return nil
		})
	}

	_ = g.Wait()
	return results
}

// updateLayout recalculates component sizes based on terminal dimensions.
func (m *Model) updateLayout() {
	helpBarHeight := 1
	editorHeight := lipgloss.Height(m.editor.View())

	statusHeight := 0
	if !m.statusLine.IsIdle() {
		statusHeight = 1
	}

	messageListHeight := m.height - helpBarHeight - editorHeight - statusHeight
	if messageListHeight < 5 {
		messageListHeight = 5
	}

	m.messageList.SetSize(m.width, messageListHeight)
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

// renderHelpBar returns a one-line help bar showing available keys.
func (m *Model) renderHelpBar() string {
	bindings := m.keys.ShortHelp()
	var parts []string
	for _, b := range bindings {
		h := b.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Join(parts, "  "))
}

type toolResult struct {
	ID      string
	Name    string
	Content string
	Err     error
}
