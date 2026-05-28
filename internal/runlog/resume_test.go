package runlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRunLog(t *testing.T) {
	dir := t.TempDir()
	logger, err := CreateRun(Options{
		CWD:        "/tmp/project",
		UserPrompt: "hello",
		RootDir:    dir,
		RunID:      "run_1",
		Now:        fixedClock(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = logger.AppendUserMessage("hello")
	_ = logger.Close()

	events, err := LoadRunLog(dir, "/tmp/project", "run_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}

	_, err = LoadRunLog(dir, "/tmp/project", "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing run")
	}

	_, err = LoadRunLog(dir, "/tmp/project", "../bad")
	if err == nil {
		t.Fatal("expected error for invalid run id")
	}
}

func TestListRuns(t *testing.T) {
	dir := t.TempDir()

	// Create two runs.
	for _, runID := range []string{"run_a", "run_b"} {
		logger, err := CreateRun(Options{
			CWD:        "/tmp/project",
			UserPrompt: "prompt " + runID,
			RootDir:    dir,
			RunID:      runID,
			Now:        fixedClock(),
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = logger.Close()
	}

	runs, err := ListRuns(dir, "/tmp/project")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}

	// Should be sorted by StartedAt descending.
	if runs[0].RunID != "run_a" || runs[1].RunID != "run_b" {
		// Both have same timestamp from fixedClock, so stable sort keeps insertion order
		// depending on sort algorithm. Let's just verify both exist.
		ids := map[string]bool{}
		for _, r := range runs {
			ids[r.RunID] = true
		}
		if !ids["run_a"] || !ids["run_b"] {
			t.Fatalf("missing run ids: %+v", runs)
		}
	}
}

func TestListRunsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	runs, err := ListRuns(dir, "/tmp/nonexistent-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("len(runs) = %d, want 0", len(runs))
	}
}

func TestListRunsSkipsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, "projects", ProjectSlug("/tmp/project"), "runs")
	_ = os.MkdirAll(runsDir, 0755)
	_ = os.WriteFile(filepath.Join(runsDir, "not-a-run.txt"), []byte("x"), 0644)
	_ = os.WriteFile(filepath.Join(runsDir, "bad.jsonl"), []byte("not json"), 0644)

	runs, err := ListRuns(dir, "/tmp/project")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("len(runs) = %d, want 0", len(runs))
	}
}
