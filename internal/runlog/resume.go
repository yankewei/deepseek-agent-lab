package runlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunInfo is a summary of a resumable run.
type RunInfo struct {
	RunID      string
	StartedAt  string
	UserPrompt string
	Status     string
}

// LoadRunLog reads and parses a run log file by run ID.
func LoadRunLog(rootDir, cwd, runID string) ([]map[string]any, error) {
	if !runIDPattern.MatchString(runID) {
		return nil, fmt.Errorf("invalid run id: %s", runID)
	}
	path := RunLogPath(rootDir, cwd, runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read run log: %w", err)
	}
	return ReadEvents(string(data))
}

// ListRuns scans the current project's runs directory and returns summaries.
func ListRuns(rootDir, cwd string) ([]RunInfo, error) {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		rootDir = filepath.Join(home, ".disco")
	}
	runsDir := filepath.Join(rootDir, "projects", ProjectSlug(cwd), "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read runs directory: %w", err)
	}

	var runs []RunInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		runID := strings.TrimSuffix(entry.Name(), ".jsonl")
		events, err := LoadRunLog(rootDir, cwd, runID)
		if err != nil {
			continue
		}

		info := RunInfo{RunID: runID}
		for _, ev := range events {
			typ, _ := ev["type"].(string)
			switch typ {
			case "session_meta":
				info.StartedAt = str(ev["startedAt"])
				info.UserPrompt = str(ev["userPrompt"])
				info.Status = str(ev["status"])
			case "run_status_changed":
				info.Status = str(ev["status"])
			}
		}
		runs = append(runs, info)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt > runs[j].StartedAt
	})
	return runs, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
