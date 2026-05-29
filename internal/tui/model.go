package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	"github.com/yankewei/ds-coding-agent/internal/skills"
	"github.com/yankewei/ds-coding-agent/internal/tools"
	"github.com/yankewei/ds-coding-agent/internal/tui/slashcmd"
)

var (
	// inputBoxStyle wraps the text input with a visible border.
	inputBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
)

// Model is the Bubble Tea model for the agent TUI.
type Model struct {
	width, height int
	contentWidth  int

	messageList *MessageList
	editor      textinput.Model
	statusLine  *StatusLine
	keys        KeyMap
	slashIndex  int
	slashHidden bool
	slashValue  string

	// Agent config
	client       *openai.Client
	modelName    string
	basePrompt   string
	systemPrompt string
	skills       []skills.Skill
	registry     *tools.Registry
	tracker      *execution.Tracker
	approval     approval.Prompt
	runLogger    *runlog.Logger
	toolDefs     []llm.ToolDefinition

	// Markdown renderer for assistant messages.
	renderer      *glamour.TermRenderer
	rendererStyle string

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
	// Style the input so text is clearly visible.
	styles := textinput.DefaultStyles(true)
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styles.Cursor.Color = lipgloss.Color("0")
	ti.SetStyles(styles)

	var toolDefs []llm.ToolDefinition
	for _, t := range registry.All() {
		toolDefs = append(toolDefs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	style := os.Getenv("GLAMOUR_STYLE")
	if style == "" {
		style = "notty"
	}
	renderer := newMarkdownRenderer(80, style)
	ml := NewMessageList()
	ml.SetRenderer(renderer)
	return &Model{
		editor:         ti,
		messageList:    ml,
		statusLine:     NewStatusLine(),
		keys:           DefaultKeyMap(),
		client:         client,
		modelName:      modelName,
		basePrompt:     systemPrompt,
		systemPrompt:   systemPrompt,
		registry:       registry,
		tracker:        tracker,
		approval:       &approval.NoOpPrompt{},
		runLogger:      logger,
		toolDefs:       toolDefs,
		renderer:       renderer,
		rendererStyle:  style,
		toolCallInputs: make(map[string]string),
		initialPrompt:  initialPrompt,
	}
}

// SetSkills configures the skills available for automatic prompt injection.
func (m *Model) SetSkills(all []skills.Skill) {
	m.skills = append([]skills.Skill(nil), all...)
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
	if strings.HasPrefix(text, "skill:") {
		return m.handleSkillCommand(text)
	}
	m.refreshSystemPrompt(text)
	m.isRunning = true
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
		for _, skill := range m.skills {
			lines = append(lines, fmt.Sprintf("  skill:%s — %s", skill.Name, skill.Description))
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

// handleSkillCommand processes a skill: command by injecting the skill into the system prompt.
func (m *Model) handleSkillCommand(text string) tea.Cmd {
	parts := strings.SplitN(text, " ", 2)
	skillName := strings.TrimPrefix(parts[0], "skill:")

	var found *skills.Skill
	for i := range m.skills {
		if m.skills[i].Name == skillName {
			found = &m.skills[i]
			break
		}
	}

	if found == nil {
		m.messageList.Add(Message{Type: MsgError, Content: "未知 skill: " + skillName, Status: StatusError})
		m.editor.Reset()
		return nil
	}

	m.systemPrompt = skills.Inject(m.basePrompt, []skills.Skill{*found})

	if len(parts) == 1 {
		m.messageList.Add(Message{Type: MsgSystem, Content: fmt.Sprintf("已激活 skill: %s", found.Name), Status: StatusDone})
		m.editor.Reset()
		return nil
	}

	content := parts[1]
	m.isRunning = true
	m.messages = append(m.messages, llm.Message{Role: "user", Content: content})
	m.messageList.Add(Message{Type: MsgUser, Content: content, Status: StatusDone})
	m.recordRunLog(m.runLogger.AppendUserMessage(content))
	m.recordRunLog(m.runLogger.AppendRunStatus("running"))
	m.statusLine.SetMode(ModeStreaming)
	m.editor.Reset()
	m.turnCtx, m.cancelTurn = context.WithCancel(context.Background())
	return m.startStreamCmd()
}

func (m *Model) matchedSlashCommands() []slashcmd.Command {
	value := m.editor.Value()
	if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "skill:") {
		return nil
	}
	lower := strings.ToLower(value)
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

func (m *Model) slashMenuActive() bool {
	value := m.editor.Value()
	return (strings.HasPrefix(value, "/") || strings.HasPrefix(value, "skill:")) && !m.slashHidden && len(m.matchedSlashCommands()) > 0
}

func (m *Model) syncSlashMenu() {
	m.editor.ShowSuggestions = false
	value := m.editor.Value()
	if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "skill:") {
		m.slashIndex = 0
		m.slashHidden = false
		m.slashValue = ""
		return
	}
	if value != m.slashValue {
		m.slashHidden = false
		m.slashValue = value
	}
	m.clampSlashIndex()
}

func (m *Model) clampSlashIndex() {
	matches := m.matchedSlashCommands()
	if len(matches) == 0 {
		m.slashIndex = 0
		return
	}
	if m.slashIndex < 0 {
		m.slashIndex = len(matches) - 1
	}
	if m.slashIndex >= len(matches) {
		m.slashIndex = 0
	}
}

func (m *Model) moveSlashSelection(delta int) {
	matches := m.matchedSlashCommands()
	if len(matches) == 0 {
		m.slashIndex = 0
		return
	}
	m.slashIndex = (m.slashIndex + delta) % len(matches)
	if m.slashIndex < 0 {
		m.slashIndex += len(matches)
	}
}

func (m *Model) selectSlashCommand() {
	matches := m.matchedSlashCommands()
	if len(matches) == 0 {
		return
	}
	m.clampSlashIndex()
	selected := matches[m.slashIndex].Name
	m.editor.SetValue(selected)
	m.editor.CursorEnd()
	m.slashHidden = true
	m.slashValue = selected
}

func (m *Model) closeSlashMenu() {
	value := m.editor.Value()
	if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "skill:") {
		return
	}
	m.slashHidden = true
	m.slashValue = value
}

func slashCommandLabel(cmd slashcmd.Command) string {
	return strings.TrimPrefix(cmd.Name, "/")
}

func (m *Model) renderSlashCommandMenu() string {
	if !m.slashMenuActive() {
		return ""
	}
	matches := m.matchedSlashCommands()
	total := len(matches)

	const maxVisible = 8
	startIdx := 0
	if total > maxVisible {
		if m.slashIndex >= maxVisible {
			startIdx = m.slashIndex - maxVisible + 1
		}
		if startIdx+maxVisible > total {
			startIdx = total - maxVisible
		}
	}
	endIdx := startIdx + maxVisible
	if endIdx > total {
		endIdx = total
	}

	lines := make([]string, 0, endIdx-startIdx)
	itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("6")).
		Bold(true)
	ellipsisStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i := startIdx; i < endIdx; i++ {
		cmd := matches[i]
		label := slashCommandLabel(cmd)
		line := fmt.Sprintf("  %s  %s", label, cmd.Description)
		if i == m.slashIndex {
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
	if m.systemPrompt != "" {
		out = append(out, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	for _, msg := range m.messages {
		if msg.Role == "system" {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func (m *Model) refreshSystemPrompt(text string) {
	m.systemPrompt = skills.Inject(m.basePrompt, skills.Match(m.skills, text))
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
	innerWidth := m.contentWidth - inputBoxStyle.GetHorizontalFrameSize()
	if innerWidth < 1 {
		innerWidth = 1
	}
	m.editor.SetWidth(innerWidth)
	helpBarHeight := 1
	editorHeight := lipgloss.Height(m.renderEditor())
	menuHeight := lipgloss.Height(m.renderSlashCommandMenu())
	statusHeight := 0
	if !m.statusLine.IsIdle() {
		statusHeight = 1
	}
	messageListHeight := m.height - helpBarHeight - editorHeight - menuHeight - statusHeight
	if messageListHeight < 5 {
		messageListHeight = 5
	}
	m.messageList.SetSize(m.contentWidth, messageListHeight)
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
	style := os.Getenv("GLAMOUR_STYLE")
	if style == "" {
		style = m.rendererStyle
	}
	renderer := newMarkdownRenderer(m.contentWidth, style)
	if renderer == nil {
		return
	}
	m.renderer = renderer
	m.rendererStyle = style
	m.messageList.SetRenderer(renderer)
	m.messageList.ClearRenderCache()
}
func newMarkdownRenderer(width int, style string) *glamour.TermRenderer {
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
	m.refreshSystemPrompt(lastUserMessage(m.messages))

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
			if msg.ReasoningContent != "" {
				m.messageList.Add(Message{
					Type:    MsgThinking,
					Content: msg.ReasoningContent,
					Status:  StatusDone,
				})
			}
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

func lastUserMessage(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
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

// renderEditor returns the editor wrapped in a visible border.
func (m *Model) renderEditor() string {
	width := m.contentWidth
	if width <= 0 {
		width = boundedContentWidth(m.width)
	}
	return inputBoxStyle.Width(width).Render(m.editor.View())
}

func (m *Model) renderHelpBar() string {
	bindings := m.keys.ShortHelp()
	var parts []string
	for _, b := range bindings {
		h := b.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Join(parts, "  "))
}
