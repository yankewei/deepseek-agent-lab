package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"context"
	"errors"
	"fmt"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
	"github.com/yankewei/ds-coding-agent/internal/skills"
	"github.com/yankewei/ds-coding-agent/internal/tools"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCtrlCCancelsRunningTurn(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	ctx, cancel := context.WithCancel(context.Background())
	m.isRunning = true
	m.turnCtx = ctx
	m.cancelTurn = cancel
	m.cancelStream = cancel

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 'c'}))
	got := updated.(*Model)

	if ctx.Err() == nil {
		t.Fatal("turn context was not canceled")
	}
	if got.cancelTurn != nil {
		t.Fatal("cancelTurn should be cleared after ctrl+c")
	}
	if got.cancelStream != nil {
		t.Fatal("cancelStream should be cleared after ctrl+c")
	}
}

func TestCanceledToolResultsEndTurn(t *testing.T) {
	logger := createTUITestLogger(t)
	m := NewModelWithLogger(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "", logger)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.isRunning = true
	m.turnCtx = ctx
	m.cancelTurn = cancel
	m.cancelStream = cancel

	updated, _ := m.Update(toolResultsMsg{results: []tools.Result{
		{ID: "call-1", Name: "testTool", Content: `{"error":"context canceled"}`, Err: errors.New("context canceled")},
	}})
	got := updated.(*Model)

	if got.isRunning {
		t.Fatal("model should stop running after canceled tool results")
	}
	if got.turnCtx != nil {
		t.Fatal("turnCtx should be cleared after canceled tool results")
	}
	if got.cancelTurn != nil {
		t.Fatal("cancelTurn should be cleared after canceled tool results")
	}
	if got.statusLine.mode != ModeIdle {
		t.Fatalf("status mode = %v, want idle", got.statusLine.mode)
	}

	events := readTUILogEvents(t, logger.Path())
	if events[len(events)-1]["status"] != "interrupted" {
		t.Fatalf("status = %v, want interrupted", events[len(events)-1]["status"])
	}
}

func TestModelPersistsUserModelAndStatusEvents(t *testing.T) {
	logger := createTUITestLogger(t)
	m := NewModelWithLogger(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "", logger)

	updated, _ := m.Update(userSubmittedMsg{text: "inspect project"})
	m = updated.(*Model)
	updated, _ = m.Update(streamStartedMsg{events: closedEventStream(), cancel: func() {}})
	m = updated.(*Model)
	updated, _ = m.Update(streamEventMsg{event: llm.EventReasoningDelta{Text: "thinking"}})
	m = updated.(*Model)
	updated, _ = m.Update(streamEventMsg{event: llm.EventTextDelta{Content: "done"}})
	m = updated.(*Model)
	updated, _ = m.Update(streamEventMsg{event: llm.EventFinish{FinishReason: "stop", Usage: llm.Usage{TotalTokens: 3}}})
	m = updated.(*Model)
	updated, _ = m.Update(streamDoneMsg{})
	m = updated.(*Model)

	if m.isRunning {
		t.Fatal("model should stop after final stream")
	}

	events := readTUILogEvents(t, logger.Path())
	got := eventTypes(events)
	want := []string{
		"session_meta",
		"user_message",
		"run_status_changed",
		"model_stream_started",
		"model_reasoning",
		"model_text",
		"model_stream_finished",
		"run_status_changed",
	}
	if !sameStrings(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	if events[7]["status"] != "completed" {
		t.Fatalf("final status = %v, want completed", events[7]["status"])
	}
}

func TestMessageListScrollSurvivesUpdateLayout(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = updated.(*Model)

	for i := 0; i < 20; i++ {
		m.messageList.Add(Message{Type: MsgAssistant, Content: fmt.Sprintf("line %02d", i), Status: StatusDone})
	}

	bottom := m.messageList.YOffset()
	if bottom == 0 {
		t.Fatal("test setup should overflow the message list")
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	m = updated.(*Model)

	if got := m.messageList.YOffset(); got >= bottom {
		t.Fatalf("YOffset = %d, want less than bottom %d", got, bottom)
	}
}

func TestMessagesForRequestInjectsSystemAndPreservesReasoning(t *testing.T) {
	m := NewModel(nil, "", "system prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.messages = []llm.Message{
		{Role: "assistant", Content: "ok", ReasoningContent: "hidden"},
	}

	got := m.messagesForRequest()
	if len(got) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(got))
	}
	if got[0].Role != "system" || got[0].Content != "system prompt" {
		t.Fatalf("first message = %+v, want system prompt", got[0])
	}
	if got[1].ReasoningContent != "hidden" {
		t.Fatalf("ReasoningContent = %q, want hidden", got[1].ReasoningContent)
	}
	if m.messages[0].ReasoningContent != "hidden" {
		t.Fatalf("messagesForRequest mutated model messages")
	}
}

func TestMessagesForRequestDoesNotDuplicateSystem(t *testing.T) {
	m := NewModel(nil, "", "system prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.messages = []llm.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
	}

	got := m.messagesForRequest()
	if len(got) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(got))
	}
	if got[0].Role != "system" || got[1].Role != "user" {
		t.Fatalf("messages = %+v", got)
	}
}

func TestSubmitInjectsMatchingSkillIntoSystemPrompt(t *testing.T) {
	m := NewModel(nil, "", "base prompt\n\n## AGENTS.md Instructions\n\nproject rules", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "write", Title: "Write", Description: "Rewrite prose", Content: "# Write\n\nUse concise prose."},
	})

	_ = m.submit("please rewrite this paragraph")

	got := m.messagesForRequest()
	if len(got) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(got))
	}
	if got[0].Role != "system" {
		t.Fatalf("first role = %q, want system", got[0].Role)
	}
	if !strings.Contains(got[0].Content, "## Active Skills") {
		t.Fatalf("system prompt = %q, want active skills", got[0].Content)
	}
	if !strings.Contains(got[0].Content, "# Write") {
		t.Fatalf("system prompt = %q, want write skill", got[0].Content)
	}
	if !strings.Contains(got[0].Content, "project rules") {
		t.Fatalf("system prompt = %q, want AGENTS.md instructions", got[0].Content)
	}
}

func TestSubmitWithoutMatchingSkillKeepsBaseSystemPrompt(t *testing.T) {
	m := NewModel(nil, "", "base prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "write", Title: "Write", Description: "Rewrite prose", Content: "# Write\n\nUse concise prose."},
	})

	_ = m.submit("inspect the project")

	got := m.messagesForRequest()
	if got[0].Content != "base prompt" {
		t.Fatalf("system prompt = %q, want base prompt", got[0].Content)
	}
}

func TestSlashClearStillAllowsSkillReinjection(t *testing.T) {
	m := NewModel(nil, "", "base prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "hunt", Title: "Hunt", Description: "Debug failures", Content: "# Hunt\n\nFind root cause first."},
	})

	_ = m.handleSlashCommand("/clear")
	_ = m.submit("debug this failure")

	got := m.messagesForRequest()
	if !strings.Contains(got[0].Content, "# Hunt") {
		t.Fatalf("system prompt = %q, want hunt skill after clear", got[0].Content)
	}
}

func TestResumeInitCreatesCancelableTurnContext(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.ResumeFrom(&runlog.Snapshot{NextAction: runlog.ActionReadyForNextStep})
	_ = m.Init()

	if !m.isRunning {
		t.Fatal("model should be running after auto-resume init")
	}
	if m.turnCtx == nil {
		t.Fatal("turnCtx should be created for resumed continuation")
	}
	if m.cancelTurn == nil {
		t.Fatal("cancelTurn should be created for resumed continuation")
	}
}

func TestSlashClearPersistsRunLogEvent(t *testing.T) {
	logger := createTUITestLogger(t)
	m := NewModelWithLogger(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "", logger)
	m.messages = []llm.Message{{Role: "user", Content: "before"}}

	_ = m.handleSlashCommand("/clear")

	if len(m.messages) != 0 {
		t.Fatalf("len(messages) = %d, want 0", len(m.messages))
	}
	events := readTUILogEvents(t, logger.Path())
	got := eventTypes(events)
	want := []string{"session_meta", "conversation_cleared"}
	if !sameStrings(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
}

func TestSlashCommandMenuMatchesAndRendersCommands(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.editor.SetValue("/")
	m.syncSlashMenu()

	matches := m.matchedSlashCommands()
	if len(matches) != 4 {
		t.Fatalf("len(matches) = %d, want 4", len(matches))
	}
	menu := m.renderSlashCommandMenu()
	for _, want := range []string{"clear", "help", "model", "quit"} {
		if !strings.Contains(menu, want) {
			t.Fatalf("menu = %q, want it to contain %q", menu, want)
		}
	}
	for _, unwanted := range []string{"/clear", "/help", "/quit"} {
		if strings.Contains(menu, unwanted) {
			t.Fatalf("menu = %q, should not contain slash-prefixed candidate %q", menu, unwanted)
		}
	}
	if !m.slashMenuActive() {
		t.Fatal("slash menu should be active for /")
	}
	if m.editor.ShowSuggestions {
		t.Fatal("textinput suggestions should stay disabled")
	}
}

func TestSlashCommandMenuFiltersByPrefix(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.editor.SetValue("/h")
	m.syncSlashMenu()

	matches := m.matchedSlashCommands()
	if len(matches) != 1 || matches[0].Name != "/help" {
		t.Fatalf("matches = %+v, want only /help", matches)
	}
	if strings.Contains(m.renderSlashCommandMenu(), "/clear") {
		t.Fatalf("menu = %q, should not contain /clear", m.renderSlashCommandMenu())
	}
}

func TestSlashCommandMenuNavigatesAndSelects(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")

	updated, _ := m.Update(keyPress("/"))
	m = updated.(*Model)
	if got := m.editor.Value(); got != "/" {
		t.Fatalf("editor value = %q, want /", got)
	}
	if m.slashIndex != 0 {
		t.Fatalf("slashIndex = %d, want 0", m.slashIndex)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(*Model)
	if m.slashIndex != 1 {
		t.Fatalf("slashIndex = %d, want 1", m.slashIndex)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	m = updated.(*Model)
	if m.slashIndex != 0 {
		t.Fatalf("slashIndex = %d, want 0", m.slashIndex)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	m = updated.(*Model)
	if m.slashIndex != 3 {
		t.Fatalf("slashIndex = %d, want 3", m.slashIndex)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(*Model)
	if got := m.editor.Value(); got != "/quit" {
		t.Fatalf("editor value = %q, want /quit", got)
	}
	if m.slashMenuActive() {
		t.Fatal("slash menu should hide after selecting a command")
	}
}

func TestSlashCommandMenuTabSelectsFilteredCommand(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.editor.SetValue("/h")
	m.syncSlashMenu()

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = updated.(*Model)
	if got := m.editor.Value(); got != "/help" {
		t.Fatalf("editor value = %q, want /help", got)
	}
	if m.slashMenuActive() {
		t.Fatal("slash menu should hide after tab selection")
	}
}

func TestSlashCommandMenuEscapeAndPlainInput(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.editor.SetValue("/")
	m.syncSlashMenu()

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	m = updated.(*Model)
	if got := m.editor.Value(); got != "/" {
		t.Fatalf("editor value = %q, want /", got)
	}
	if m.slashMenuActive() {
		t.Fatal("slash menu should be closed after escape")
	}

	m.editor.SetValue("hello")
	m.syncSlashMenu()
	if m.slashMenuActive() {
		t.Fatal("slash menu should not be active for plain input")
	}
	if menu := m.renderSlashCommandMenu(); menu != "" {
		t.Fatalf("menu = %q, want empty", menu)
	}
}

func createTUITestLogger(t *testing.T) *runlog.Logger {
	t.Helper()
	logger, err := runlog.CreateRun(runlog.Options{
		CWD:        t.TempDir(),
		UserPrompt: "test",
		RootDir:    t.TempDir(),
		RunID:      "run_1",
		Now: func() time.Time {
			return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}

func keyPress(text string) tea.KeyPressMsg {
	runes := []rune(text)
	if len(runes) == 0 {
		return tea.KeyPressMsg{}
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
}

func closedEventStream() <-chan llm.Event {
	ch := make(chan llm.Event)
	close(ch)
	return ch
}

func readTUILogEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	events, err := runlog.ReadEvents(string(data))
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func eventTypes(events []map[string]any) []string {
	out := make([]string, len(events))
	for i, event := range events {
		out[i], _ = event["type"].(string)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSlashCommandMenuIncludesSkills(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "read", Title: "Read", Description: "Fetch URLs"},
		{Name: "write", Title: "Write", Description: "Rewrite prose"},
	})

	// 输入 / 时，菜单同时包含固定命令和 skill 命令
	m.editor.SetValue("/")
	m.syncSlashMenu()
	matches := m.matchedSlashCommands()
	if len(matches) != 6 {
		t.Fatalf("len(matches) = %d, want 6 (4 fixed + 2 skills)", len(matches))
	}
	menu := m.renderSlashCommandMenu()
	for _, want := range []string{"clear", "help", "model", "quit", "skill:read", "skill:write"} {
		if !strings.Contains(menu, want) {
			t.Fatalf("menu = %q, want it to contain %q", menu, want)
		}
	}
	if !m.slashMenuActive() {
		t.Fatal("slash menu should be active for /")
	}

	// 输入 skill: 时，只显示 skill 命令
	m.editor.SetValue("skill:")
	m.syncSlashMenu()
	matches = m.matchedSlashCommands()
	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(matches))
	}
}

func TestSlashCommandMenuFiltersSkillsByPrefix(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "read", Title: "Read", Description: "Fetch URLs"},
		{Name: "write", Title: "Write", Description: "Rewrite prose"},
	})
	m.editor.SetValue("skill:r")
	m.syncSlashMenu()

	matches := m.matchedSlashCommands()
	if len(matches) != 1 || matches[0].Name != "skill:read" {
		t.Fatalf("matches = %+v, want only skill:read", matches)
	}
}

func TestSlashCommandMenuFiltersSkillsBySlashQuery(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "read", Title: "Read", Description: "Fetch URLs"},
		{Name: "write", Title: "Write", Description: "Rewrite prose"},
	})

	m.editor.SetValue("/read")
	m.syncSlashMenu()
	matches := m.matchedSlashCommands()
	if len(matches) != 1 || matches[0].Name != "skill:read" {
		t.Fatalf("matches = %+v, want only skill:read for /read", matches)
	}

	m.editor.SetValue("/r")
	m.syncSlashMenu()
	matches = m.matchedSlashCommands()
	if len(matches) != 1 || matches[0].Name != "skill:read" {
		t.Fatalf("matches = %+v, want only skill:read for /r", matches)
	}

	m.editor.SetValue("/w")
	m.syncSlashMenu()
	matches = m.matchedSlashCommands()
	if len(matches) != 1 || matches[0].Name != "skill:write" {
		t.Fatalf("matches = %+v, want only skill:write for /w", matches)
	}
}

func TestSkillCommandActivatesSkillWithoutMessage(t *testing.T) {
	m := NewModel(nil, "", "base prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "read", Title: "Read", Description: "Fetch URLs", Content: "# Read\n\nFetch any URL."},
	})

	_ = m.handleSkillCommand("skill:read")

	if !strings.Contains(m.systemPrompt, "## Active Skills") {
		t.Fatalf("system prompt = %q, want active skills", m.systemPrompt)
	}
	if !strings.Contains(m.systemPrompt, "# Read") {
		t.Fatalf("system prompt = %q, want read skill", m.systemPrompt)
	}
	if len(m.messages) != 0 {
		t.Fatalf("len(messages) = %d, want 0", len(m.messages))
	}
}

func TestSkillCommandActivatesSkillWithMessage(t *testing.T) {
	m := NewModel(nil, "", "base prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "read", Title: "Read", Description: "Fetch URLs", Content: "# Read\n\nFetch any URL."},
	})

	_ = m.handleSkillCommand("skill:read hello world")

	if !strings.Contains(m.systemPrompt, "# Read") {
		t.Fatalf("system prompt = %q, want read skill", m.systemPrompt)
	}
	if len(m.messages) != 1 || m.messages[0].Content != "hello world" {
		t.Fatalf("messages = %+v, want one user message with 'hello world'", m.messages)
	}
}

func TestSkillCommandUnknownSkill(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.SetSkills([]skills.Skill{
		{Name: "read", Title: "Read", Description: "Fetch URLs"},
	})

	_ = m.handleSkillCommand("skill:unknown")

	if len(m.messages) != 0 {
		t.Fatalf("len(messages) = %d, want 0", len(m.messages))
	}
}
func TestModelRebuildsRendererOnWidthChange(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	oldRenderer := m.renderer
	if oldRenderer == nil {
		t.Fatal("expected renderer to be set after initial layout")
	}
	// Trigger a width change
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(*Model)
	if m.renderer == oldRenderer {
		t.Fatal("expected renderer to be rebuilt on width change, got same pointer")
	}
}
func TestModelBackgroundColorAppliesAutoTheme(t *testing.T) {
	t.Setenv("GLAMOUR_STYLE", "")
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	oldRenderer := m.renderer
	if oldRenderer == nil {
		t.Fatal("expected renderer to be set after initial layout")
	}
	// Send BackgroundColorMsg with dark background
	updated, _ = m.Update(tea.BackgroundColorMsg{Color: color.RGBA{0, 0, 0, 255}})
	m = updated.(*Model)
	if m.renderer == oldRenderer {
		t.Fatal("expected renderer to change after BackgroundColorMsg")
	}
	if m.rendererStyle != "dark" {
		t.Fatalf("expected rendererStyle to be dark, got %s", m.rendererStyle)
	}
}
func TestModelBackgroundColorRespectsGlamourStyleOverride(t *testing.T) {
	t.Setenv("GLAMOUR_STYLE", "dracula")
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	oldRenderer := m.renderer
	if oldRenderer == nil {
		t.Fatal("expected renderer to be set after initial layout")
	}
	// BackgroundColorMsg should be ignored when GLAMOUR_STYLE is set
	updated, _ = m.Update(tea.BackgroundColorMsg{Color: color.RGBA{0, 0, 0, 255}})
	m = updated.(*Model)
	if m.renderer != oldRenderer {
		t.Fatal("expected renderer to stay unchanged when GLAMOUR_STYLE is set")
	}
}
func TestCursorColorIsBlack(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	r, g, b, _ := m.editor.Styles().Cursor.Color.RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Fatalf("cursor color not black, got RGBA(%d, %d, %d)", r, g, b)
	}
}
func TestRenderEditorHasBorder(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	raw := m.editor.View()
	bordered := m.renderEditor()
	if lipgloss.Height(bordered) <= lipgloss.Height(raw) {
		t.Fatalf("bordered editor height %d should be greater than raw editor height %d", lipgloss.Height(bordered), lipgloss.Height(raw))
	}
	// Check for at least one rounded-border corner character.
	if !strings.Contains(bordered, "╰") {
		t.Fatalf("renderEditor output should contain rounded border corner, got:\n%s", bordered)
	}
}
func TestRenderStatusLineHasBorder(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	bordered := m.renderStatusLine()
	if lipgloss.Height(bordered) <= 1 {
		t.Fatalf("bordered status line height %d should be greater than 1", lipgloss.Height(bordered))
	}
	if !strings.Contains(bordered, "╭") {
		t.Fatalf("renderStatusLine output should contain rounded border corner, got:\n%s", bordered)
	}
}
func TestStatusLineShowsModelName(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	rendered := m.renderStatusLine()
	if !strings.Contains(rendered, "deepseek-chat") {
		t.Fatalf("status line should contain model name, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "ctx") {
		t.Fatalf("status line should not show ctx when no tokens, got:\n%s", rendered)
	}
}
func TestStatusLineShowsEstimatedInitialContextTokens(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "system prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(*Model)
	rendered := m.renderStatusLine()
	if !strings.Contains(rendered, "ctx ~") {
		t.Fatalf("status line should show estimated ctx before the first response, got:\n%s", rendered)
	}
}
func TestStatusLineShowsContextTokens(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(*Model)
	m.statusLine.SetContextTokens(1234)
	rendered := m.renderStatusLine()
	if !strings.Contains(rendered, "deepseek-chat") {
		t.Fatalf("status line should contain model name, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "ctx 1.2k/1.0M (0.1%)") {
		t.Fatalf("status line should show formatted ctx, got:\n%s", rendered)
	}
}
func TestStatusLineReplacesEstimateWithExactUsage(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "system prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(streamEventMsg{event: llm.EventFinish{Usage: llm.Usage{PromptTokens: 1234}}})
	m = updated.(*Model)
	rendered := m.renderStatusLine()
	if !strings.Contains(rendered, "ctx 1.2k/1.0M (0.1%)") {
		t.Fatalf("status line should show exact ctx after usage arrives, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "ctx ~") {
		t.Fatalf("status line should remove the estimate marker after usage arrives, got:\n%s", rendered)
	}
}
func TestStatusLineShowsContextTokensWithoutPercentForUnknownModel(t *testing.T) {
	m := NewModel(nil, "custom-model", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(*Model)
	m.statusLine.SetContextTokens(1234)
	rendered := m.renderStatusLine()
	if !strings.Contains(rendered, "ctx 1.2k") {
		t.Fatalf("status line should show formatted ctx, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "%") {
		t.Fatalf("status line should not show percent for unknown model, got:\n%s", rendered)
	}
}
func TestStatusLineShowsTinyContextPercent(t *testing.T) {
	m := NewModel(nil, "deepseek-v4-flash", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(*Model)
	m.statusLine.SetContextTokens(1)
	rendered := m.renderStatusLine()
	if !strings.Contains(rendered, "ctx 1/1.0M (<0.1%)") {
		t.Fatalf("status line should show tiny ctx percent, got:\n%s", rendered)
	}
}
func TestFooterDoesNotRenderActivityText(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(*Model)
	m.statusLine.SetMode(ModeStreaming)

	view := m.View().Content
	for _, text := range []string{"Responding...", "Thinking...", "Executing..."} {
		if strings.Contains(view, text) {
			t.Fatalf("footer should not contain activity text %q, got:\n%s", text, view)
		}
	}
	if !strings.Contains(view, "deepseek-chat") {
		t.Fatalf("view should still contain status line model, got:\n%s", view)
	}
}
func TestIdleActivityDoesNotConsumeLayoutHeight(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(*Model)
	if activity := m.statusLine.RenderActivity(); activity != "" {
		t.Fatalf("idle activity should be empty, got:\n%s", activity)
	}
}

func TestUpdateLayoutAccountsForActivityLine(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.statusLine.SetMode(ModeStreaming)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(*Model)

	expectedListHeight := m.currentMessageListHeight()
	if m.messageList.height != expectedListHeight {
		t.Fatalf("message list height = %d, want %d", m.messageList.height, expectedListHeight)
	}
}
func TestUpdateLayoutAccountsForBorderedInput(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	expectedListHeight := m.currentMessageListHeight()
	if m.messageList.height != expectedListHeight {
		t.Fatalf("message list height = %d, want %d", m.messageList.height, expectedListHeight)
	}
	innerWidth := m.contentWidth - inputBoxStyle.GetHorizontalFrameSize()
	if innerWidth < 1 {
		innerWidth = 1
	}
	if m.editor.Width() != innerWidth {
		t.Fatalf("editor width = %d, want %d (inner width after border frame)", m.editor.Width(), innerWidth)
	}
}
func TestViewEnablesMouseModeByDefault(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	v := m.View()
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("MouseMode = %v, want MouseModeCellMotion by default", v.MouseMode)
	}
}

func TestViewDisablesMouseModeAfterToggle(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 'm'}))
	m = updated.(*Model)
	v := m.View()
	if v.MouseMode != tea.MouseModeNone {
		t.Fatalf("MouseMode = %v, want MouseModeNone after toggle", v.MouseMode)
	}
}

func TestViewReEnablesMouseModeAfterSecondToggle(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m = updated.(*Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 'm'}))
	m = updated.(*Model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 'm'}))
	m = updated.(*Model)
	v := m.View()
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("MouseMode = %v, want MouseModeCellMotion after second toggle", v.MouseMode)
	}
}
func TestViewAnchorsStatusLineAndEditorAtBottom(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(*Model)

	view := m.View().Content
	if got := lipgloss.Height(view); got != 20 {
		t.Fatalf("view height = %d, want 20", got)
	}
	if !strings.Contains(view, "deepseek-chat") {
		t.Fatalf("view should contain status line, got:\n%s", view)
	}
	if !strings.Contains(view, ">") {
		t.Fatalf("view should contain editor prompt, got:\n%s", view)
	}
}

func TestViewDoesNotOverflowSmallTerminal(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = updated.(*Model)
	for i := 0; i < 20; i++ {
		m.messageList.Add(Message{Type: MsgAssistant, Content: fmt.Sprintf("line %02d", i), Status: StatusDone})
	}

	view := m.View().Content
	if got := lipgloss.Height(view); got > 8 {
		t.Fatalf("view height = %d, want <= 8", got)
	}
	if !strings.Contains(view, "deepseek-chat") {
		t.Fatalf("view should contain status line, got:\n%s", view)
	}
	if !strings.Contains(view, ">") {
		t.Fatalf("view should contain editor prompt, got:\n%s", view)
	}
}

func TestStreamingTextDeltaAutoScrollsMessageList(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = updated.(*Model)
	updated, _ = m.Update(userSubmittedMsg{text: "hello"})
	m = updated.(*Model)
	updated, _ = m.Update(streamStartedMsg{events: closedEventStream(), cancel: func() {}})
	m = updated.(*Model)

	for i := 0; i < 20; i++ {
		updated, _ = m.Update(streamEventMsg{event: llm.EventTextDelta{Content: fmt.Sprintf("line %02d\n", i)}})
		m = updated.(*Model)
	}

	if !m.messageList.AtBottomHeight(m.currentMessageListHeight()) {
		t.Fatalf("message list should stay at bottom during streaming, offset=%d", m.messageList.YOffset())
	}
	view := m.View().Content
	if !strings.Contains(view, "line 19") {
		t.Fatalf("view should show latest streamed content, got:\n%s", view)
	}
}

func TestStreamingThinkingDeltaAutoScrollsMessageList(t *testing.T) {
	m := NewModel(nil, "deepseek-chat", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = updated.(*Model)
	updated, _ = m.Update(userSubmittedMsg{text: "hello"})
	m = updated.(*Model)
	updated, _ = m.Update(streamStartedMsg{events: closedEventStream(), cancel: func() {}})
	m = updated.(*Model)

	for i := 0; i < 20; i++ {
		updated, _ = m.Update(streamEventMsg{event: llm.EventReasoningDelta{Text: fmt.Sprintf("step-%02d ", i)}})
		m = updated.(*Model)
	}

	if !m.messageList.AtBottomHeight(m.currentMessageListHeight()) {
		t.Fatalf("message list should stay at bottom during thinking, offset=%d", m.messageList.YOffset())
	}
	view := m.View().Content
	if !strings.Contains(view, "Thinking") {
		t.Fatalf("view should show thinking indicator, got:\n%s", view)
	}
	if strings.Contains(view, "step-19") {
		t.Fatalf("view should hide thinking content, got:\n%s", view)
	}
}
func TestModelReasoningDeltaCreatesThinkingRow(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(userSubmittedMsg{text: "hello"})
	m = updated.(*Model)
	updated, _ = m.Update(streamStartedMsg{events: closedEventStream(), cancel: func() {}})
	m = updated.(*Model)
	updated, _ = m.Update(streamEventMsg{event: llm.EventReasoningDelta{Text: "step one"}})
	m = updated.(*Model)
	msgs := m.messageList.Messages()
	var thinkingMsgs []Message
	for _, msg := range msgs {
		if msg.Type == MsgThinking {
			thinkingMsgs = append(thinkingMsgs, msg)
		}
	}
	if len(thinkingMsgs) != 1 {
		t.Fatalf("expected 1 thinking message, got %d", len(thinkingMsgs))
	}
	if thinkingMsgs[0].Content != "step one" {
		t.Fatalf("expected thinking content 'step one', got %q", thinkingMsgs[0].Content)
	}
	if thinkingMsgs[0].Status != StatusStreaming {
		t.Fatalf("expected thinking status streaming, got %v", thinkingMsgs[0].Status)
	}
	updated, _ = m.Update(streamEventMsg{event: llm.EventReasoningDelta{Text: " step two"}})
	m = updated.(*Model)
	msgs = m.messageList.Messages()
	thinkingMsgs = nil
	for _, msg := range msgs {
		if msg.Type == MsgThinking {
			thinkingMsgs = append(thinkingMsgs, msg)
		}
	}
	if len(thinkingMsgs) != 1 {
		t.Fatalf("expected 1 thinking message after update, got %d", len(thinkingMsgs))
	}
	if thinkingMsgs[0].Content != "step one step two" {
		t.Fatalf("expected thinking content 'step one step two', got %q", thinkingMsgs[0].Content)
	}
	updated, _ = m.Update(streamEventMsg{event: llm.EventFinish{FinishReason: "stop"}})
	m = updated.(*Model)
	thinking := m.messageList.Find(MsgThinking, StatusDone)
	if thinking == nil {
		t.Fatal("expected thinking message to be marked done after finish")
	}
}
func TestResumeCreatesThinkingRowFromReasoningContent(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.ResumeFrom(&runlog.Snapshot{
		Messages: []llm.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi", ReasoningContent: "let me think"},
		},
		NextAction: runlog.ActionReadyForNextStep,
	})
	msgs := m.messageList.Messages()
	var foundThinking bool
	for _, msg := range msgs {
		if msg.Type == MsgThinking {
			foundThinking = true
			if msg.Content != "let me think" {
				t.Fatalf("expected thinking content 'let me think', got %q", msg.Content)
			}
			if msg.Status != StatusDone {
				t.Fatalf("expected thinking status done, got %v", msg.Status)
			}
		}
	}
	if !foundThinking {
		t.Fatal("expected a thinking message after resume")
	}
}

func TestMessageListMouseWheelScrolls(t *testing.T) {
	m := NewModel(nil, "", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = updated.(*Model)
	for i := 0; i < 20; i++ {
		m.messageList.Add(Message{Type: MsgAssistant, Content: fmt.Sprintf("line %02d", i), Status: StatusDone})
	}
	bottom := m.messageList.YOffset()
	if bottom == 0 {
		t.Fatal("test setup should overflow the message list")
	}
	updated, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	m = updated.(*Model)
	if got := m.messageList.YOffset(); got >= bottom {
		t.Fatalf("YOffset = %d, want less than bottom %d after wheel-up", got, bottom)
	}
}

func TestModelSlashCommandOpensSelector(t *testing.T) {
	m := NewModel(nil, "deepseek-v4-flash", "", tools.NewRegistry(), execution.NewTracker(nil), "")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})

	cmd := m.submit("/model")
	if cmd == nil {
		t.Fatal("submit /model should return a cmd")
	}
	if m.form == nil {
		t.Fatal("form should be set after /model command")
	}
	if m.formKind != "model" {
		t.Fatalf("formKind = %q, want \"model\"", m.formKind)
	}
}
