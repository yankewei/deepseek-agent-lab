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
	Name          string            `json:"name"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	WhenToUse     string            `json:"whenToUse,omitempty"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	AllowedTools  string            `json:"allowedTools,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Path          string            `json:"path"`
}

func (t *listSkillsTool) Name() string   { return "listSkills" }
func (t *listSkillsTool) Effect() Effect { return EffectRead }
func (t *listSkillsTool) Description() string {
	return "List loaded skills and their metadata"
}
func (t *listSkillsTool) Schema() map[string]any {
	return objectSchema(map[string]any{})
}

func (t *listSkillsTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	if err := decodeInput(input, &struct{}{}); err != nil {
		return nil, err
	}
	out := make([]listedSkill, 0, len(t.skills))
	for _, skill := range t.skills {
		out = append(out, listedSkill{
			Name:          skill.Name,
			Title:         skill.Title,
			Description:   skill.Description,
			WhenToUse:     skill.WhenToUse,
			License:       skill.License,
			Compatibility: skill.Compatibility,
			AllowedTools:  skill.AllowedTools,
			Metadata:      skill.Metadata,
			Path:          skill.Path,
		})
	}
	return out, nil
}

func NewListSkillsTool(all []skills.Skill) Tool {
	return &listSkillsTool{skills: append([]skills.Skill(nil), all...)}
}
