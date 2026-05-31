package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Result contains the prompt fragment built from discovered AGENTS.md files
// and any non-fatal read errors.
type Result struct {
	Prompt   string
	Warnings []error
}

// Load reads user-level and git-root-level AGENTS.md files.
func Load(gitRoot, homeDir string) Result {
	type candidate struct {
		title string
		path  string
	}
	var candidates []candidate
	if homeDir != "" {
		candidates = append(candidates, candidate{title: "User Instructions: ~/AGENTS.md", path: filepath.Join(homeDir, "AGENTS.md")})
	}
	candidates = append(candidates, candidate{title: "Project Instructions: " + filepath.Join(gitRoot, "AGENTS.md"), path: filepath.Join(gitRoot, "AGENTS.md")})

	var result Result
	var sections []string
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate.path == "" {
			continue
		}
		path := filepath.Clean(candidate.path)
		if seen[path] {
			continue
		}
		seen[path] = true

		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Errorf("read %s: %w", path, err))
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		sections = append(sections, fmt.Sprintf("### %s\n%s", candidate.title, content))
	}
	if len(sections) > 0 {
		result.Prompt = "## AGENTS.md Instructions\n\n" + strings.Join(sections, "\n\n")
	}
	return result
}

// Append adds loaded instructions to the built-in system prompt.
func Append(base, loaded string) string {
	if strings.TrimSpace(loaded) == "" {
		return base
	}
	return strings.TrimRight(base, "\n") + "\n\n" + loaded
}
