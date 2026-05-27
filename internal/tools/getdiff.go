package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type getDiffTool struct{}

func (t *getDiffTool) Name() string        { return "getDiff" }
func (t *getDiffTool) Effect() Effect      { return EffectRead }
func (t *getDiffTool) Description() string { return "Show the current git diff" }
func (t *getDiffTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *getDiffTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	cmd := exec.CommandContext(ctx, "git", "diff")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w\n%s", err, string(out))
	}
	return string(out), nil
}

func NewGetDiffTool() Tool { return &getDiffTool{} }
