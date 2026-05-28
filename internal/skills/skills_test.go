package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadScansProjectAndAgentsSkills(t *testing.T) {
	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	writeSkill(t, filepath.Join(projectRoot, ".disco", "skills", "project-skill"), "# Project Skill\n\nProject description.")
	writeSkill(t, filepath.Join(homeDir, ".agents", "skills", "agent-skill"), "# Agent Skill\n\nAgent description.")
	writeSkill(t, filepath.Join(homeDir, ".disco", "skills", "ignored"), "# Ignored\n\nShould not load.")

	got, err := Load(DefaultRoots(projectRoot, homeDir, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len(skills) = %d, want 2: %+v", len(got), got)
	}
	if !hasSkill(got, "project-skill") {
		t.Fatal("project skill was not loaded")
	}
	if !hasSkill(got, "agent-skill") {
		t.Fatal("agents skill was not loaded")
	}
	if hasSkill(got, "ignored") {
		t.Fatal("~/.disco skill should not be loaded")
	}
}

func TestLoadProjectSkillOverridesAgentsSkill(t *testing.T) {
	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	writeSkill(t, filepath.Join(homeDir, ".agents", "skills", "write"), "# User Write\n\nUser description.")
	writeSkill(t, filepath.Join(projectRoot, ".disco", "skills", "write"), "# Project Write\n\nProject description.")

	got, err := Load(DefaultRoots(projectRoot, homeDir, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].Title != "Project Write" {
		t.Fatalf("Title = %q, want project override", got[0].Title)
	}
}

func TestLoadParsesFrontmatterAndMatchesWhenToUse(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, filepath.Join(dir, "write"), `---
name: write
description: "Rewrite prose naturally."
when_to_use: "帮我写, polish, rewrite"
---

# Write

Body.`)

	got, err := Load([]Root{{Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].Description != "Rewrite prose naturally." {
		t.Fatalf("Description = %q", got[0].Description)
	}
	if got[0].WhenToUse != "帮我写, polish, rewrite" {
		t.Fatalf("WhenToUse = %q", got[0].WhenToUse)
	}

	matches := Match(got, "帮我写一段介绍")
	if len(matches) != 1 || matches[0].Name != "write" {
		t.Fatalf("matches = %+v, want write", matches)
	}
}

func TestInjectAddsOnlyMatchedSkills(t *testing.T) {
	skills := []Skill{
		{Name: "write", Title: "Write", Description: "Rewrite prose", Content: "# Write\n\nRules.", Path: "/tmp/skills/write/SKILL.md"},
		{Name: "hunt", Title: "Hunt", Description: "Debug failures", Content: "# Hunt\n\nRules.", Path: "/tmp/skills/hunt/SKILL.md"},
	}
	matches := Match(skills, "please rewrite this")
	got := Inject("base prompt", matches)

	if !strings.Contains(got, "## Active Skills") {
		t.Fatalf("prompt = %q, want active skills section", got)
	}
	if !strings.Contains(got, "# Write") {
		t.Fatalf("prompt = %q, want write skill", got)
	}
	if !strings.Contains(got, "Skill directory: /tmp/skills/write") {
		t.Fatalf("prompt = %q, want skill directory", got)
	}
	if strings.Contains(got, "# Hunt") {
		t.Fatalf("prompt = %q, should not include hunt skill", got)
	}
}

func TestLoadSkipsMissingEmptyAndBadEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := Load([]Root{{Path: filepath.Join(dir, "missing")}, {Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len(skills) = %d, want 0", len(got))
	}
}

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func hasSkill(skills []Skill, name string) bool {
	for _, skill := range skills {
		if skill.Name == name {
			return true
		}
	}
	return false
}
