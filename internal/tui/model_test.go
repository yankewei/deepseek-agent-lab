package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
	"github.com/yankewei/ds-coding-agent/internal/skills"
	"github.com/yankewei/ds-coding-agent/internal/tools"
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

	bottom := m.messageList.viewport.YOffset()
	if bottom == 0 {
		t.Fatal("test setup should overflow the message list")
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	m = updated.(*Model)

	if got := m.messageList.viewport.YOffset(); got >= bottom {
		t.Fatalf("YOffset = %d, want less than bottom %d", got, bottom)
	}
}

func TestMessagesForRequestInjectsSystemAndStripsReasoning(t *testing.T) {
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
	if got[1].ReasoningContent != "" {
		t.Fatalf("ReasoningContent = %q, want empty", got[1].ReasoningContent)
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
	m := NewModel(nil, "", "base prompt", tools.NewRegistry(), execution.NewTracker(nil), "")
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
	if len(matches) != 3 {
		t.Fatalf("len(matches) = %d, want 3", len(matches))
	}
	menu := m.renderSlashCommandMenu()
	for _, want := range []string{"clear", "help", "quit"} {
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
	if m.slashIndex != 2 {
		t.Fatalf("slashIndex = %d, want 2", m.slashIndex)
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
