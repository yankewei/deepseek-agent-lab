package execution

import (
	"testing"
	"time"
)

func TestTrackerLifecycle(t *testing.T) {
	var events []Event
	tr := NewTracker(func(ev Event) {
		events = append(events, ev)
	})

	rec := tr.CreateRecord("command", "", "bun test", "run tests")
	if rec.Status != StatusCreated {
		t.Fatalf("expected status created, got %s", rec.Status)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	tr.UpdateRecord(rec.ID, map[string]any{"status": StatusRunning})
	tr.UpdateRecord(rec.ID, map[string]any{"status": StatusCompleted, "exit_code": 0})

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	final := tr.GetRecord(rec.ID)
	if final.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", final.Status)
	}
	if final.ExitCode == nil || *final.ExitCode != 0 {
		t.Error("expected exit code 0")
	}
	if final.DurationMs == nil || *final.DurationMs < 0 {
		t.Error("expected positive duration")
	}
}

func TestTrackerTerminalStatus(t *testing.T) {
	tr := NewTracker(nil)
	rec := tr.CreateRecord("tool", "listFiles", "", "")
	tr.UpdateRecord(rec.ID, map[string]any{"status": StatusFailed, "error": "boom"})

	got := tr.GetRecord(rec.ID)
	if got.Status != StatusFailed {
		t.Errorf("expected failed, got %s", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("expected completed_at to be set on terminal status")
	}
}

func TestTrackerListRecords(t *testing.T) {
	tr := NewTracker(nil)
	tr.CreateRecord("command", "", "pwd", "")
	tr.CreateRecord("tool", "readFile", "", "")
	if len(tr.ListRecords()) != 2 {
		t.Errorf("expected 2 records, got %d", len(tr.ListRecords()))
	}
}

func TestTrackerUpdateMissingRecord(t *testing.T) {
	tr := NewTracker(nil)
	got := tr.UpdateRecord("nonexistent", map[string]any{"status": StatusRunning})
	if got != nil {
		t.Error("expected nil for missing record")
	}
}

func TestCloneRecord(t *testing.T) {
	rec := &Record{
		ID:      "1",
		Status:  StatusCreated,
		History: []HistoryEntry{{Status: StatusCreated, At: time.Now()}},
	}
	cp := cloneRecord(rec)
	cp.History[0].Status = StatusRunning
	if rec.History[0].Status != StatusCreated {
		t.Error("clone should be independent")
	}
}
