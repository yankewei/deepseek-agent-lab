package tools

import (
	"context"
	"encoding/json"

	"github.com/yankewei/ds-coding-agent/internal/skills"
)

type listSkillsTool struct {
	skills []skills.Skill
}

type listedSkill struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	WhenToUse   string `json:"whenToUse,omitempty"`
	Path        string `json:"path"`
}

func (t *listSkillsTool) Name() string   { return "listSkills" }
func (t *listSkillsTool) Effect() Effect { return EffectRead }
func (t *listSkillsTool) Description() string {
	return "List loaded skills and their metadata"
}
func (t *listSkillsTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *listSkillsTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	out := make([]listedSkill, 0, len(t.skills))
	for _, skill := range t.skills {
		out = append(out, listedSkill{
			Name:        skill.Name,
			Title:       skill.Title,
			Description: skill.Description,
			WhenToUse:   skill.WhenToUse,
			Path:        skill.Path,
		})
	}
	return out, nil
}

func NewListSkillsTool(all []skills.Skill) Tool {
	return &listSkillsTool{skills: append([]skills.Skill(nil), all...)}
}
