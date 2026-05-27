package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
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
