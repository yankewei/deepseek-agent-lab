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
	writeSkill(t, filepath.Join(projectRoot, ".disco", "skills", "project-skill"), "---\nname: project-skill\ndescription: Project desc.\n---\n\n# Project Skill\n\nProject description.")
	writeSkill(t, filepath.Join(homeDir, ".agents", "skills", "agent-skill"), "---\nname: agent-skill\ndescription: Agent desc.\n---\n\n# Agent Skill\n\nAgent description.")
	// This dir should be ignored because ~/.disco/skills is not a root.
	writeSkill(t, filepath.Join(homeDir, ".disco", "skills", "ignored"), "---\nname: ignored\ndescription: Ignored.\n---\n\n# Ignored\n\nShould not load.")

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
	writeSkill(t, filepath.Join(homeDir, ".agents", "skills", "write"), "---\nname: write\ndescription: User.\n---\n\n# User Write\n\nUser description.")
	writeSkill(t, filepath.Join(projectRoot, ".disco", "skills", "write"), "---\nname: write\ndescription: Project.\n---\n\n# Project Write\n\nProject description.")

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
	if got[0].Content != "" {
		t.Fatalf("Content should be empty after Load (lazy), got %q", got[0].Content)
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

func TestValidateNameAcceptsValidNames(t *testing.T) {
	cases := []string{"a", "check", "code-review", "pdf-processing", "a1-b2-c3"}
	for _, name := range cases {
		if err := ValidateName(name, name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateNameRejectsInvalidNames(t *testing.T) {
	cases := []struct {
		name    string
		dirName string
		want    string
	}{
		{"", "foo", "required"},
		{"Check", "Check", "lowercase"},
		{"-pdf", "-pdf", "start"},
		{"pdf-", "pdf-", "end"},
		{"pdf--processing", "pdf--processing", "consecutive"},
		{"pdf_processing", "pdf_processing", "only contain"},
		{"this-name-is-way-too-long-because-it-exceeds-sixty-four-characters-total", "this-name-is-way-too-long-because-it-exceeds-sixty-four-characters-total", "64"},
		{"write", "rewrite", "match directory"},
	}
	for _, tc := range cases {
		err := ValidateName(tc.name, tc.dirName)
		if err == nil {
			t.Errorf("ValidateName(%q, %q) = nil, want error containing %q", tc.name, tc.dirName, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ValidateName(%q, %q) error = %q, want containing %q", tc.name, tc.dirName, err.Error(), tc.want)
		}
	}
}

func TestLoadSkipsInvalidSkillName(t *testing.T) {
	dir := t.TempDir()
	// Invalid: uppercase in name
	writeSkill(t, filepath.Join(dir, "Bad-Name"), "---\nname: Bad-Name\ndescription: Bad.\n---\n\n# Bad\n\nBody.")
	// Valid
	writeSkill(t, filepath.Join(dir, "good-name"), "---\nname: good-name\ndescription: Good.\n---\n\n# Good\n\nBody.")

	got, err := Load([]Root{{Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].Name != "good-name" {
		t.Fatalf("Name = %q, want good-name", got[0].Name)
	}
}

func TestSkillLazyBodyLoad(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "lazy-skill")
	writeSkill(t, skillDir, "---\nname: lazy-skill\ndescription: Lazy.\n---\n\n# Lazy Skill\n\nThis is the body.")

	got, err := Load([]Root{{Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].Content != "" {
		t.Fatal("Content should be empty before Body() is called")
	}

	body, err := got[0].Body()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "This is the body.") {
		t.Fatalf("Body = %q, want containing 'This is the body.'", body)
	}
	if strings.Contains(body, "---") {
		t.Fatalf("Body should not contain frontmatter delimiters: %q", body)
	}

	// Second call should use cached Content without re-reading disk.
	body2, err := got[0].Body()
	if err != nil {
		t.Fatal(err)
	}
	if body2 != body {
		t.Fatalf("Body() returned different content on second call")
	}
}

func TestLoadParsesNestedMetadata(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, filepath.Join(dir, "meta-skill"), `---
name: meta-skill
description: "Meta."
metadata:
  author: example-org
  version: "1.0"
---

# Meta Skill

Body.`)

	got, err := Load([]Root{{Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].Metadata == nil {
		t.Fatal("Metadata is nil")
	}
	if got[0].Metadata["author"] != "example-org" {
		t.Fatalf("author = %q, want example-org", got[0].Metadata["author"])
	}
	if got[0].Metadata["version"] != "1.0" {
		t.Fatalf("version = %q, want 1.0", got[0].Metadata["version"])
	}
}

func TestLoadParsesLicenseCompatibilityAllowedTools(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, filepath.Join(dir, "full-skill"), `---
name: full-skill
description: "Full."
license: Apache-2.0
compatibility: Requires git and docker
allowed-tools: Bash(git:*) Read
---

# Full Skill

Body.`)

	got, err := Load([]Root{{Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].License != "Apache-2.0" {
		t.Fatalf("License = %q, want Apache-2.0", got[0].License)
	}
	if got[0].Compatibility != "Requires git and docker" {
		t.Fatalf("Compatibility = %q", got[0].Compatibility)
	}
	if got[0].AllowedTools != "Bash(git:*) Read" {
		t.Fatalf("AllowedTools = %q", got[0].AllowedTools)
	}
}

func TestSkillWithoutFrontmatterStillLoads(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, filepath.Join(dir, "plain-skill"), "# Plain Skill\n\nPlain description.")

	got, err := Load([]Root{{Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].Name != "plain-skill" {
		t.Fatalf("Name = %q, want plain-skill", got[0].Name)
	}
	if got[0].Title != "Plain Skill" {
		t.Fatalf("Title = %q, want Plain Skill", got[0].Title)
	}
	if got[0].Description != "Plain description." {
		t.Fatalf("Description = %q, want Plain description.", got[0].Description)
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
