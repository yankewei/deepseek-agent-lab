package runlog

import (
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/execution"
)

func TestBuildSnapshotEmpty(t *testing.T) {
	s, err := BuildSnapshot(nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.NextAction != ActionInterrupted {
		t.Fatalf("NextAction = %q, want interrupted", s.NextAction)
	}
}

func TestBuildSnapshotUserOnly(t *testing.T) {
	events := []map[string]any{
		{"type": "session_meta", "runId": "run_1", "cwd": "/tmp/project", "userPrompt": "hello", "status": "running"},
		{"type": "user_message", "text": "hello"},
	}
	s, err := BuildSnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(s.Messages))
	}
	if s.Messages[0].Role != "user" || s.Messages[0].Content != "hello" {
		t.Fatalf("first message = %+v", s.Messages[0])
	}
	if s.NextAction != ActionInterrupted {
		t.Fatalf("NextAction = %q, want interrupted", s.NextAction)
	}
}

func TestBuildSnapshotAssistantNoTools(t *testing.T) {
	events := []map[string]any{
		{"type": "session_meta", "runId": "run_1", "cwd": "/tmp/project", "userPrompt": "hello", "status": "running"},
		{"type": "user_message", "text": "hello"},
		{"type": "model_stream_started"},
		{"type": "model_text", "text": "Hi there"},
		{"type": "model_stream_finished", "finishReason": "stop"},
	}
	s, err := BuildSnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(s.Messages))
	}
	if s.Messages[1].Role != "assistant" || s.Messages[1].Content != "Hi there" {
		t.Fatalf("assistant message = %+v", s.Messages[1])
	}
	if s.NextAction != ActionCompleted {
		t.Fatalf("NextAction = %q, want completed", s.NextAction)
	}
}

func TestBuildSnapshotWithToolCall(t *testing.T) {
	events := []map[string]any{
		{"type": "session_meta", "runId": "run_1", "cwd": "/tmp/project", "userPrompt": "hello", "status": "running"},
		{"type": "user_message", "text": "hello"},
		{"type": "model_stream_started"},
		{"type": "model_text", "text": "Let me check"},
		{"type": "model_stream_finished", "finishReason": "tool_calls"},
		{"type": "tool_call", "toolCallId": "call_1", "toolName": "listFiles", "input": map[string]any{"path": "."}},
		{"type": "execution_state_changed", "record": map[string]any{"id": "exec-1", "kind": "tool", "tool_name": "listFiles", "status": "completed", "started_at": "2026-01-01T00:00:00Z", "history": []any{}}},
		{"type": "tool_result", "toolCallId": "call_1", "toolName": "listFiles", "output": []any{"a.go", "b.go"}},
	}
	s, err := BuildSnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(s.Messages))
	}
	if s.Messages[1].Role != "assistant" {
		t.Fatalf("message[1].Role = %q, want assistant", s.Messages[1].Role)
	}
	if len(s.Messages[1].ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(s.Messages[1].ToolCalls))
	}
	if s.Messages[1].ToolCalls[0].Name != "listFiles" {
		t.Fatalf("ToolCall.Name = %q, want listFiles", s.Messages[1].ToolCalls[0].Name)
	}
	if s.Messages[2].Role != "tool" || s.Messages[2].ToolCallID != "call_1" {
		t.Fatalf("message[2] = %+v", s.Messages[2])
	}
	if s.NextAction != ActionReadyForNextStep {
		t.Fatalf("NextAction = %q, want ready_for_next_model_step", s.NextAction)
	}
	if len(s.Executions) != 1 {
		t.Fatalf("len(Executions) = %d, want 1", len(s.Executions))
	}
}

func TestBuildSnapshotMultipleTurns(t *testing.T) {
	events := []map[string]any{
		{"type": "user_message", "text": "first"},
		{"type": "model_stream_started"},
		{"type": "model_text", "text": "A"},
		{"type": "model_stream_finished", "finishReason": "stop"},
		{"type": "user_message", "text": "second"},
		{"type": "model_stream_started"},
		{"type": "model_text", "text": "B"},
		{"type": "model_stream_finished", "finishReason": "stop"},
	}
	s, err := BuildSnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 4 {
		t.Fatalf("len(Messages) = %d, want 4", len(s.Messages))
	}
	if s.Messages[0].Content != "first" {
		t.Fatalf("msg[0] = %q", s.Messages[0].Content)
	}
	if s.Messages[1].Content != "A" {
		t.Fatalf("msg[1] = %q", s.Messages[1].Content)
	}
	if s.Messages[2].Content != "second" {
		t.Fatalf("msg[2] = %q", s.Messages[2].Content)
	}
	if s.Messages[3].Content != "B" {
		t.Fatalf("msg[3] = %q", s.Messages[3].Content)
	}
}

func TestBuildSnapshotRunStatusChanged(t *testing.T) {
	events := []map[string]any{
		{"type": "session_meta", "runId": "run_1", "cwd": "/tmp/project", "userPrompt": "hello", "status": "running"},
		{"type": "user_message", "text": "hello"},
		{"type": "model_stream_started"},
		{"type": "model_text", "text": "ok"},
		{"type": "model_stream_finished", "finishReason": "stop"},
		{"type": "run_status_changed", "status": "completed"},
	}
	s, err := BuildSnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	if s.NextAction != ActionCompleted {
		t.Fatalf("NextAction = %q, want completed", s.NextAction)
	}
	if s.Status != "running" {
		// session_meta status is kept as-is; run_status_changed drives NextAction
		t.Fatalf("Status = %q, want running (from session_meta)", s.Status)
	}
}

func TestBuildSnapshotFailedRun(t *testing.T) {
	events := []map[string]any{
		{"type": "user_message", "text": "hello"},
		{"type": "run_status_changed", "status": "failed"},
	}
	s, err := BuildSnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	if s.NextAction != ActionFailed {
		t.Fatalf("NextAction = %q, want failed", s.NextAction)
	}
}

func TestBuildSnapshotExecutionRecords(t *testing.T) {
	events := []map[string]any{
		{"type": "execution_state_changed", "record": map[string]any{
			"id": "rec-1", "kind": "command", "command": "go test", "status": "running",
			"started_at": "2026-01-01T00:00:00Z", "history": []any{},
		}},
		{"type": "execution_state_changed", "record": map[string]any{
			"id": "rec-1", "kind": "command", "command": "go test", "status": "completed",
			"started_at": "2026-01-01T00:00:00Z", "history": []any{},
		}},
		{"type": "execution_state_changed", "record": map[string]any{
			"id": "rec-2", "kind": "tool", "tool_name": "readFile", "status": "completed",
			"started_at": "2026-01-01T00:00:00Z", "history": []any{},
		}},
	}
	s, err := BuildSnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Executions) != 2 {
		t.Fatalf("len(Executions) = %d, want 2", len(s.Executions))
	}
	if s.Executions["rec-1"].Status != execution.StatusCompleted {
		t.Fatalf("rec-1 status = %q, want completed", s.Executions["rec-1"].Status)
	}
	if s.Executions["rec-2"].Status != execution.StatusCompleted {
		t.Fatalf("rec-2 status = %q, want completed", s.Executions["rec-2"].Status)
	}
}
