package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type gitStatusTool struct{}

func (t *gitStatusTool) Name() string        { return "gitStatus" }
func (t *gitStatusTool) Effect() Effect      { return EffectRead }
func (t *gitStatusTool) Description() string { return "Show the current git working tree status" }
func (t *gitStatusTool) Schema() map[string]any {
	return objectSchema(map[string]any{})
}

func (t *gitStatusTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	if err := decodeInput(input, &struct{}{}); err != nil {
		return nil, err
	}
	result, err := runCapturedCommand(ctx, "git", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w\n%s", err, result.Stderr)
	}
	return result.Stdout, nil
}

func NewGitStatusTool() Tool { return &gitStatusTool{} }
