package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/sashabaranov/go-openai"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
	"github.com/yankewei/ds-coding-agent/internal/tools"
	"github.com/yankewei/ds-coding-agent/internal/tui/slashcmd"
)

// Model is the Bubble Tea model for the agent TUI.
type Model struct {
	width, height int
	contentWidth  int

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
	approval     approval.Prompt
	runLogger    *runlog.Logger
	toolDefs     []llm.ToolDefinition

	// Markdown renderer for assistant messages.
	renderer *glamour.TermRenderer

	// Conversation state
	messages         []llm.Message
	isRunning        bool
	eventStream      <-chan llm.Event
	turnCtx          context.Context
	cancelTurn       context.CancelFunc
	cancelStream     context.CancelFunc
	thinkingBuf      string
	pendingToolCalls []llm.ToolCallDef
	toolCallInputs   map[string]string // toolID -> argsJSON
	finishReason     string
	finishUsage      llm.Usage
	streamFailed     bool

	// Optional initial prompt to auto-start on init.
	initialPrompt string

	// Resume state.
	resumeSnapshot *runlog.Snapshot

	// Pending approval state.
	approvalReq   *approval.Request
	approvalResCh chan approval.Result
	form          *huh.Form
}

// NewModel creates a new TUI model.
func NewModel(client *openai.Client, modelName, systemPrompt string, registry *tools.Registry, tracker *execution.Tracker, initialPrompt string) *Model {
	return NewModelWithLogger(client, modelName, systemPrompt, registry, tracker, initialPrompt, nil)
}

// NewModelWithLogger creates a new TUI model with optional run logging.
func NewModelWithLogger(client *openai.Client, modelName, systemPrompt string, registry *tools.Registry, tracker *execution.Tracker, initialPrompt string, logger *runlog.Logger) *Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = ""
	ti.Focus()
	ti.SetWidth(80)

	// Bind suggestion navigation to pgup/pgdown.
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("pgdown"))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("pgup"))

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

	renderer := newMarkdownRenderer(80)

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
		approval:       &approval.NoOpPrompt{},
		runLogger:      logger,
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
	if m.resumeSnapshot != nil && m.resumeSnapshot.NextAction == runlog.ActionReadyForNextStep {
		m.isRunning = true
		m.turnCtx, m.cancelTurn = context.WithCancel(context.Background())
		m.statusLine.SetMode(ModeStreaming)
		cmds = append(cmds, func() tea.Msg {
			return resumeContinueMsg{}
		})
	} else if m.initialPrompt != "" {
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
	if strings.HasPrefix(text, "/") {
		return m.handleSlashCommand(text)
	}
	m.isRunning = true
	if m.systemPrompt != "" && len(m.messages) == 0 {
		m.messages = append(m.messages, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	m.messages = append(m.messages, llm.Message{Role: "user", Content: text})
	m.messageList.Add(Message{Type: MsgUser, Content: text, Status: StatusDone})
	m.recordRunLog(m.runLogger.AppendUserMessage(text))
	m.recordRunLog(m.runLogger.AppendRunStatus("running"))
	m.statusLine.SetMode(ModeStreaming)
	m.editor.Reset()
	m.turnCtx, m.cancelTurn = context.WithCancel(context.Background())
	return m.startStreamCmd()
}

// handleSlashCommand processes a slash command locally without sending to the LLM.
func (m *Model) handleSlashCommand(text string) tea.Cmd {
	switch text {
	case "/clear":
		m.messages = nil
		m.messageList = NewMessageList()
		m.messageList.SetRenderer(m.renderer)
		if m.width > 0 {
			m.updateLayout()
		}
		m.toolCallInputs = make(map[string]string)
		m.thinkingBuf = ""
		m.pendingToolCalls = nil
		m.statusLine.SetMode(ModeIdle)
		m.recordRunLog(m.runLogger.AppendConversationCleared())
		m.messageList.Add(Message{Type: MsgSystem, Content: "对话已清除", Status: StatusDone})
		m.editor.Reset()
		return nil
	case "/help":
		var lines []string
		lines = append(lines, "可用命令：")
		for _, cmd := range slashcmd.All() {
			lines = append(lines, fmt.Sprintf("  %s — %s", cmd.Name, cmd.Description))
		}
		m.messageList.Add(Message{Type: MsgSystem, Content: strings.Join(lines, "\n"), Status: StatusDone})
		m.editor.Reset()
		return nil
	case "/quit":
		return tea.Quit
	default:
		m.messageList.Add(Message{Type: MsgError, Content: "未知命令: " + text, Status: StatusError})
		m.editor.Reset()
		return nil
	}
}

// startStreamCmd initiates the LLM stream with a cancellable context.
func (m *Model) startStreamCmd() tea.Cmd {
	ctx := m.turnCtx
	if ctx == nil {
		ctx = context.Background()
	}
	messages := m.messagesForRequest()
	toolDefs := append([]llm.ToolDefinition(nil), m.toolDefs...)
	return func() tea.Msg {
		events, err := llm.Stream(ctx, m.client, m.modelName, messages, toolDefs)
		if err != nil {
			return errorMsg{err: err}
		}
		return streamStartedMsg{events: events, cancel: m.cancelTurn}
	}
}

func (m *Model) messagesForRequest() []llm.Message {
	out := make([]llm.Message, 0, len(m.messages)+1)
	if m.systemPrompt != "" && (len(m.messages) == 0 || m.messages[0].Role != "system") {
		out = append(out, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	for _, msg := range m.messages {
		msg.ReasoningContent = ""
		out = append(out, msg)
	}
	return out
}

// executeToolsCmd runs the given tool calls in parallel.
func (m *Model) executeToolsCmd(calls []llm.ToolCallDef) tea.Cmd {
	ctx := m.turnCtx
	if ctx == nil {
		ctx = context.Background()
	}
	return func() tea.Msg {
		toolCalls := make([]tools.Call, len(calls))
		for i, call := range calls {
			toolCalls[i] = tools.Call{ID: call.ID, Name: call.Name, Input: call.Input}
		}
		results := tools.Executor{
			Registry: m.registry,
			Tracker:  m.tracker,
			Prompt:   m.approval,
			Logger:   m.runLogger,
		}.Execute(ctx, toolCalls)
		return toolResultsMsg{results: results}
	}
}

// updateLayout recalculates component sizes based on terminal dimensions.
func (m *Model) updateLayout() {
	previousContentWidth := m.contentWidth
	m.contentWidth = boundedContentWidth(m.width)

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

	m.messageList.SetSize(m.contentWidth, messageListHeight)
	m.editor.SetWidth(m.contentWidth)
	if m.contentWidth != previousContentWidth || m.renderer == nil {
		m.rebuildRenderer()
	}
}

// SetPrompt rebuilds the tool registry with the given approval prompt.
func (m *Model) SetPrompt(prompt approval.Prompt) {
	m.approval = prompt
	m.registry = tools.CreateRegistryWithLogger(m.tracker, prompt, m.runLogger)
	m.toolDefs = nil
	for _, t := range m.registry.All() {
		m.toolDefs = append(m.toolDefs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
}

func (m *Model) recordRunLog(err error) {
	if err == nil {
		return
	}
	m.messageList.Add(Message{Type: MsgError, Content: "run log error: " + err.Error(), Status: StatusError})
}

func (m *Model) rebuildRenderer() {
	if m.contentWidth <= 0 {
		return
	}
	renderer := newMarkdownRenderer(m.contentWidth)
	if renderer == nil {
		return
	}
	m.renderer = renderer
	m.messageList.SetRenderer(renderer)
	m.messageList.refresh()
}

func newMarkdownRenderer(width int) *glamour.TermRenderer {
	style := os.Getenv("GLAMOUR_STYLE")
	if style == "" {
		style = "notty"
	}
	if width < 20 {
		width = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	return r
}

// ResumeFrom restores conversation state from a snapshot.
func (m *Model) ResumeFrom(snapshot *runlog.Snapshot) {
	m.resumeSnapshot = snapshot
	m.messages = append([]llm.Message(nil), snapshot.Messages...)

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			m.messageList.Add(Message{Type: MsgUser, Content: msg.Content, Status: StatusDone})
		case "assistant":
			m.messageList.Add(Message{
				Type:    MsgAssistant,
				Content: msg.Content,
				Status:  StatusDone,
				Metadata: map[string]any{
					"reasoning": msg.ReasoningContent,
				},
			})
			for _, tc := range msg.ToolCalls {
				m.toolCallInputs[tc.ID] = string(tc.Input)
				m.messageList.Add(Message{
					Type:   MsgToolCall,
					Status: StatusDone,
					Metadata: map[string]any{
						"tool_id":    tc.ID,
						"tool_name":  tc.Name,
						"tool_input": string(tc.Input),
					},
				})
			}
		case "tool":
			toolName := ""
			for i := len(m.messages) - 1; i >= 0; i-- {
				if m.messages[i].Role == "assistant" {
					for _, tc := range m.messages[i].ToolCalls {
						if tc.ID == msg.ToolCallID {
							toolName = tc.Name
							break
						}
					}
					break
				}
			}
			meta := map[string]any{
				"tool_name":  toolName,
				"tool_input": m.toolCallInputs[msg.ToolCallID],
				"success":    !strings.HasPrefix(msg.Content, `{"error"`),
			}
			m.messageList.Add(Message{
				Type:     MsgToolResult,
				Content:  msg.Content,
				Metadata: meta,
				Status:   StatusDone,
			})
		}
	}

	if snapshot.NextAction != runlog.ActionReadyForNextStep {
		m.messageList.Add(Message{
			Type:    MsgSystem,
			Content: fmt.Sprintf("▶ Resumed run %s (%s)", snapshot.RunID, snapshot.NextAction),
			Status:  StatusDone,
		})
	}
}

func boundedContentWidth(terminalWidth int) int {
	const rightPadding = 2
	const minContentWidth = 20
	if terminalWidth <= 0 {
		return 80
	}
	width := terminalWidth - rightPadding
	if width < minContentWidth {
		return minContentWidth
	}
	return width
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
