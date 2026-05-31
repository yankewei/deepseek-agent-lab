package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type getDiffTool struct{}

func (t *getDiffTool) Name() string        { return "getDiff" }
func (t *getDiffTool) Effect() Effect      { return EffectRead }
func (t *getDiffTool) Description() string { return "Show the current git diff" }
func (t *getDiffTool) Schema() map[string]any {
	return objectSchema(map[string]any{})
}

func (t *getDiffTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	if err := decodeInput(input, &struct{}{}); err != nil {
		return nil, err
	}
	result, err := runCapturedCommand(ctx, "git", "diff")
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w\n%s", err, result.Stderr)
	}
	return result.Stdout, nil
}

func NewGetDiffTool() Tool { return &getDiffTool{} }
