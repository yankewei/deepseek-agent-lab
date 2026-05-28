package tools

import (
	"context"
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/skills"
)

func TestListSkillsTool(t *testing.T) {
	tool := NewListSkillsTool([]skills.Skill{
		{
			Name:        "write",
			Title:       "Write",
			Description: "Rewrite prose",
			WhenToUse:   "rewrite, polish",
			Path:        "/Users/test/.agents/skills/write/SKILL.md",
			Content:     "# Write\n\nRules.",
		},
	})

	out, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := out.([]listedSkill)
	if len(got) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got))
	}
	if got[0].Name != "write" {
		t.Fatalf("Name = %q, want write", got[0].Name)
	}
	if got[0].Path != "/Users/test/.agents/skills/write/SKILL.md" {
		t.Fatalf("Path = %q", got[0].Path)
	}
}

func TestRegistryIncludesListSkills(t *testing.T) {
	registry := CreateRegistryWithLoggerAndSkills(nil, nil, nil, []skills.Skill{{Name: "write"}})
	if registry.Get("listSkills") == nil {
		t.Fatal("listSkills should be registered")
	}
}
