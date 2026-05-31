package instructions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMergesUserAndProjectRootInstructions(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "home")
	repo := filepath.Join(parent, "repo")
	mkdirAll(t, home, repo)
	writeFile(t, filepath.Join(home, "AGENTS.md"), "user rules")
	writeFile(t, filepath.Join(repo, "AGENTS.md"), "project rules")

	got := Load(repo, home)

	if len(got.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none", got.Warnings)
	}
	userIndex := strings.Index(got.Prompt, "user rules")
	projectIndex := strings.Index(got.Prompt, "project rules")
	if userIndex < 0 || projectIndex < 0 || userIndex >= projectIndex {
		t.Fatalf("prompt = %q, want user rules before project rules", got.Prompt)
	}
	if !strings.Contains(got.Prompt, "Project Instructions: "+filepath.Join(repo, "AGENTS.md")) {
		t.Fatalf("prompt = %q, want project AGENTS.md path", got.Prompt)
	}
}

func TestLoadUsesProjectRootDirectly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "AGENTS.md"), "local rules")

	got := Load(dir, "")

	if !strings.Contains(got.Prompt, "local rules") {
		t.Fatalf("prompt = %q, want local rules", got.Prompt)
	}
}

func TestLoadIgnoresMissingFiles(t *testing.T) {
	dir := t.TempDir()

	got := Load(dir, filepath.Join(dir, "missing-home"))

	if got.Prompt != "" {
		t.Fatalf("prompt = %q, want empty", got.Prompt)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none", got.Warnings)
	}
}

func TestLoadWarnsAndContinuesAfterReadFailure(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "AGENTS.md"))

	got := Load(dir, "")

	if len(got.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one warning", got.Warnings)
	}
	if !strings.Contains(got.Warnings[0].Error(), "AGENTS.md") {
		t.Fatalf("warning = %q, want AGENTS.md path", got.Warnings[0])
	}
}

func TestLoadDeduplicatesSameUserAndProjectPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "AGENTS.md"), "shared rules")

	got := Load(dir, dir)

	if count := strings.Count(got.Prompt, "shared rules"); count != 1 {
		t.Fatalf("shared rules count = %d, want 1 in prompt %q", count, got.Prompt)
	}
}

func TestAppendAddsInstructionsAfterBasePrompt(t *testing.T) {
	got := Append("base prompt\n", "## AGENTS.md Instructions\n\nrules")
	want := "base prompt\n\n## AGENTS.md Instructions\n\nrules"
	if got != want {
		t.Fatalf("Append() = %q, want %q", got, want)
	}
}

func mkdirAll(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
