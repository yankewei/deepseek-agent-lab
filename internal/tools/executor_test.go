package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/projectpath"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
)

type staticPrompt struct {
	result approval.Result
}

func (p staticPrompt) Request(ctx context.Context, req approval.Request) (approval.Result, error) {
	return p.result, nil
}

func TestExecutorApplyPatchDeleteApproval(t *testing.T) {
	t.Run("deny skips delete", func(t *testing.T) {
		dir := t.TempDir()
		projectpath.Init(dir)
		target := filepath.Join(dir, "delete-me.txt")
		if err := os.WriteFile(target, []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}

		registry := NewRegistry()
		registry.Register(NewApplyPatchTool())
		executor := Executor{
			Registry: registry,
			Tracker:  execution.NewTracker(nil),
			Prompt:   staticPrompt{result: approval.Result{Decision: "deny", Reason: "no"}},
		}
		patch := "*** Begin Patch\n*** Delete File: delete-me.txt\n*** End Patch"
		input, _ := json.Marshal(map[string]any{"patch": patch})

		results := executor.Execute(context.Background(), []Call{{ID: "call-1", Name: "applyPatch", Input: input}})
		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(results))
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("file should still exist: %v", err)
		}
		if results[0].Err != nil {
			t.Fatalf("deny should return a skipped result, got err: %v", results[0].Err)
		}
	})

	t.Run("approve deletes", func(t *testing.T) {
		dir := t.TempDir()
		projectpath.Init(dir)
		target := filepath.Join(dir, "delete-me.txt")
		if err := os.WriteFile(target, []byte("remove"), 0644); err != nil {
			t.Fatal(err)
		}

		registry := NewRegistry()
		registry.Register(NewApplyPatchTool())
		executor := Executor{
			Registry: registry,
			Tracker:  execution.NewTracker(nil),
			Prompt:   staticPrompt{result: approval.Result{Decision: "approve_once"}},
		}
		patch := "*** Begin Patch\n*** Delete File: delete-me.txt\n*** End Patch"
		input, _ := json.Marshal(map[string]any{"patch": patch})

		results := executor.Execute(context.Background(), []Call{{ID: "call-1", Name: "applyPatch", Input: input}})
		if len(results) != 1 || results[0].Err != nil {
			t.Fatalf("unexpected result: %+v", results)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("file should be deleted, stat err: %v", err)
		}
	})
}

func TestExecutorSerializesSideEffectingBatch(t *testing.T) {
	registry := NewRegistry()
	order := &orderedRecorder{}
	registry.Register(&recordingTool{name: "first", effect: EffectWrite, order: order})
	registry.Register(&recordingTool{name: "second", effect: EffectWrite, order: order})

	executor := Executor{
		Registry: registry,
		Tracker:  execution.NewTracker(nil),
		Prompt:   staticPrompt{result: approval.Result{Decision: "approve_once"}},
	}
	results := executor.Execute(context.Background(), []Call{
		{ID: "1", Name: "first", Input: []byte(`{}`)},
		{ID: "2", Name: "second", Input: []byte(`{}`)},
	})
	if len(results) != 2 || results[0].Err != nil || results[1].Err != nil {
		t.Fatalf("unexpected results: %+v", results)
	}
	got := order.values()
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("tools executed out of order: %v", got)
	}
}

func TestExecutorStopsSerialBatchAfterCancel(t *testing.T) {
	registry := NewRegistry()
	order := &orderedRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	registry.Register(&cancelingTool{name: "first", order: order, cancel: cancel})
	registry.Register(&recordingTool{name: "second", effect: EffectWrite, order: order})

	executor := Executor{
		Registry: registry,
		Tracker:  execution.NewTracker(nil),
		Prompt:   staticPrompt{result: approval.Result{Decision: "approve_once"}},
	}
	results := executor.Execute(ctx, []Call{
		{ID: "1", Name: "first", Input: []byte(`{}`)},
		{ID: "2", Name: "second", Input: []byte(`{}`)},
	})

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("first tool should complete before cancel is observed, got: %v", results[0].Err)
	}
	if results[1].Err == nil {
		t.Fatal("second tool should be canceled before execution")
	}
	got := order.values()
	if len(got) != 1 || got[0] != "first" {
		t.Fatalf("unexpected execution order after cancel: %v", got)
	}
}

func TestExecutorPersistsToolCallAndResult(t *testing.T) {
	logger := createExecutorTestLogger(t)
	registry := NewRegistry()
	registry.Register(&recordingTool{name: "readThing", effect: EffectRead, order: &orderedRecorder{}})

	executor := Executor{
		Registry: registry,
		Tracker:  execution.NewTracker(nil),
		Prompt:   staticPrompt{result: approval.Result{Decision: "approve_once"}},
		Logger:   logger,
	}
	results := executor.Execute(context.Background(), []Call{{ID: "call-1", Name: "readThing", Input: []byte(`{"path":"a.txt"}`)}})
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("unexpected results: %+v", results)
	}

	events := readExecutorLogEvents(t, logger.Path())
	got := runlogEventTypes(events)
	want := []string{"session_meta", "tool_call", "tool_result"}
	if !sameStringSlice(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	if events[2]["executionId"] == "" {
		t.Fatal("tool_result should include executionId")
	}
}

func TestExecutorPersistsApprovalEvents(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	target := filepath.Join(dir, "delete-me.txt")
	if err := os.WriteFile(target, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	logger := createExecutorTestLogger(t)
	registry := NewRegistry()
	registry.Register(NewApplyPatchTool())
	executor := Executor{
		Registry: registry,
		Tracker:  execution.NewTracker(nil),
		Prompt:   staticPrompt{result: approval.Result{Decision: "deny", Reason: "no"}},
		Logger:   logger,
	}
	patch := "*** Begin Patch\n*** Delete File: delete-me.txt\n*** End Patch"
	input, _ := json.Marshal(map[string]any{"patch": patch})

	results := executor.Execute(context.Background(), []Call{{ID: "call-1", Name: "applyPatch", Input: input}})
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("unexpected results: %+v", results)
	}

	events := readExecutorLogEvents(t, logger.Path())
	got := runlogEventTypes(events)
	want := []string{"session_meta", "tool_call", "approval_requested", "approval_resolved", "tool_result"}
	if !sameStringSlice(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	if events[2]["executionId"] == "" || events[3]["executionId"] == "" {
		t.Fatal("approval events should include executionId")
	}
}

type orderedRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *orderedRecorder) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, name)
}

func (r *orderedRecorder) values() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type recordingTool struct {
	name   string
	effect Effect
	order  *orderedRecorder
}

func (t *recordingTool) Name() string           { return t.name }
func (t *recordingTool) Effect() Effect         { return t.effect }
func (t *recordingTool) Description() string    { return t.name }
func (t *recordingTool) Schema() map[string]any { return map[string]any{"type": "object"} }
func (t *recordingTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	time.Sleep(5 * time.Millisecond)
	t.order.add(t.name)
	return map[string]string{"name": t.name}, nil
}

type cancelingTool struct {
	name   string
	order  *orderedRecorder
	cancel context.CancelFunc
}

func (t *cancelingTool) Name() string           { return t.name }
func (t *cancelingTool) Effect() Effect         { return EffectWrite }
func (t *cancelingTool) Description() string    { return t.name }
func (t *cancelingTool) Schema() map[string]any { return map[string]any{"type": "object"} }
func (t *cancelingTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	t.order.add(t.name)
	t.cancel()
	return map[string]string{"name": t.name}, nil
}

func createExecutorTestLogger(t *testing.T) *runlog.Logger {
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

func readExecutorLogEvents(t *testing.T, path string) []map[string]any {
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

func runlogEventTypes(events []map[string]any) []string {
	out := make([]string, len(events))
	for i, event := range events {
		out[i], _ = event["type"].(string)
	}
	return out
}

func sameStringSlice(a, b []string) bool {
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
