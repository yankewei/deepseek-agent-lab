package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type searchFilesTool struct{}

func (t *searchFilesTool) Name() string        { return "searchFiles" }
func (t *searchFilesTool) Effect() Effect      { return EffectRead }
func (t *searchFilesTool) Description() string { return "Search project files with ripgrep (rg)" }
func (t *searchFilesTool) Schema() map[string]any {
	return objectSchema(
		map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search pattern",
			},
		},
		"query",
	)
}

func (t *searchFilesTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := decodeInput(input, &args); err != nil {
		return nil, err
	}

	if args.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	result, err := runCapturedCommand(ctx, "rg", "-n", "--color=never", args.Query)
	if err != nil {
		// rg returns exit code 1 when no matches, which is not an error for us.
		if result.ExitCode == 1 {
			return "", nil
		}
		return nil, fmt.Errorf("rg failed: %w\n%s", err, result.Stderr)
	}

	return strings.TrimSpace(result.Stdout), nil
}

func NewSearchFilesTool() Tool { return &searchFilesTool{} }
