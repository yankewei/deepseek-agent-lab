package runlog

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/yankewei/ds-coding-agent/internal/execution"
)

func TestCreateRunWritesSessionMeta(t *testing.T) {
	dir := t.TempDir()
	now := fixedClock()

	logger, err := CreateRun(Options{
		CWD:        "/tmp/demo project",
		UserPrompt: "inspect project",
		RootDir:    dir,
		RunID:      "run_1",
		Now:        now,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	if logger.RunID() != "run_1" {
		t.Fatalf("RunID = %q, want run_1", logger.RunID())
	}
	if logger.Path() != RunLogPath(dir, "/tmp/demo project", "run_1") {
		t.Fatalf("Path = %q", logger.Path())
	}

	events := readLogEvents(t, logger.Path())
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0]["type"] != "session_meta" {
		t.Fatalf("type = %v, want session_meta", events[0]["type"])
	}
	if events[0]["runId"] != "run_1" {
		t.Fatalf("runId = %v, want run_1", events[0]["runId"])
	}
	if events[0]["userPrompt"] != "inspect project" {
		t.Fatalf("userPrompt = %v", events[0]["userPrompt"])
	}
}

func TestAppendWritesValidJSONL(t *testing.T) {
	logger := createTestLogger(t)

	if err := logger.AppendUserMessage("hello"); err != nil {
		t.Fatal(err)
	}
	if err := logger.AppendRunStatus("completed"); err != nil {
		t.Fatal(err)
	}

	events := readLogEvents(t, logger.Path())
	got := eventTypes(events)
	want := []string{"session_meta", "user_message", "run_status_changed"}
	if !sameStrings(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
}

func TestReadEventsIgnoresEmptyLines(t *testing.T) {
	events, err := ReadEvents("\n{\"type\":\"one\"}\n\n{\"type\":\"two\"}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
}

func TestCreateRunRejectsInvalidRunID(t *testing.T) {
	_, err := CreateRun(Options{
		CWD:     "/tmp/project",
		RootDir: t.TempDir(),
		RunID:   "../bad",
		Now:     fixedClock(),
	})
	if err == nil {
		t.Fatal("expected invalid run id error")
	}
}

func TestCreateRunDoesNotOverwriteExistingLog(t *testing.T) {
	opts := Options{
		CWD:     "/tmp/project",
		RootDir: t.TempDir(),
		RunID:   "run_1",
		Now:     fixedClock(),
	}
	logger, err := CreateRun(opts)
	if err != nil {
		t.Fatal(err)
	}
	_ = logger.Close()

	_, err = CreateRun(opts)
	if err == nil {
		t.Fatal("expected second create to fail")
	}
}

func TestOpenExistingAppendsToLog(t *testing.T) {
	logger := createTestLogger(t)
	_ = logger.AppendUserMessage("first")
	_ = logger.Close()

	logger2, err := OpenExisting(logger.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer logger2.Close()

	if logger2.RunID() != logger.RunID() {
		t.Fatalf("RunID = %q, want %q", logger2.RunID(), logger.RunID())
	}

	if err := logger2.AppendUserMessage("second"); err != nil {
		t.Fatal(err)
	}

	events := readLogEvents(t, logger.Path())
	got := eventTypes(events)
	want := []string{"session_meta", "user_message", "user_message"}
	if !sameStrings(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
}

func TestOpenExistingInvalidPath(t *testing.T) {
	_, err := OpenExisting("/dev/null/invalid")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestConcurrentAppendKeepsJSONLinesIntact(t *testing.T) {
	logger := createTestLogger(t)
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := logger.AppendUserMessage("hello"); err != nil {
				t.Errorf("append: %v", err)
			}
		}()
	}
	wg.Wait()

	events := readLogEvents(t, logger.Path())
	if len(events) != 51 {
		t.Fatalf("len(events) = %d, want 51", len(events))
	}
}

func TestExecutionEventsAppendToRunLog(t *testing.T) {
	logger := createTestLogger(t)
	defer logger.Close()

	tracker := execution.NewTracker(func(ev execution.Event) {
		if err := logger.AppendExecutionEvent(ev); err != nil {
			t.Errorf("append execution event: %v", err)
		}
	})
	rec := tracker.CreateRecord("tool", "readFile", "", "")
	tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusRunning})
	tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusCompleted})

	events := readLogEvents(t, logger.Path())
	got := eventTypes(events)
	want := []string{
		"session_meta",
		"execution_state_changed",
		"execution_state_changed",
		"execution_state_changed",
	}
	if !sameStrings(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	if events[1]["sequence"] != float64(1) {
		t.Fatalf("sequence = %v, want 1", events[1]["sequence"])
	}
}

func createTestLogger(t *testing.T) *Logger {
	t.Helper()
	logger, err := CreateRun(Options{
		CWD:        "/tmp/project",
		UserPrompt: "prompt",
		RootDir:    t.TempDir(),
		RunID:      "run_1",
		Now:        fixedClock(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}

func readLogEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	events, err := ReadEvents(string(data))
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func fixedClock() Clock {
	return func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC)
	}
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
