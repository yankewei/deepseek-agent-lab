package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type gitStatusTool struct{}

func (t *gitStatusTool) Name() string        { return "gitStatus" }
func (t *gitStatusTool) Effect() Effect      { return EffectRead }
func (t *gitStatusTool) Description() string { return "Show the current git working tree status" }
func (t *gitStatusTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *gitStatusTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w\n%s", err, string(out))
	}
	return string(out), nil
}

func NewGitStatusTool() Tool { return &gitStatusTool{} }
