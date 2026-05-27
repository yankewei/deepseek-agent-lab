package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type searchFilesTool struct{}

func (t *searchFilesTool) Name() string        { return "searchFiles" }
func (t *searchFilesTool) Description() string { return "Search project files with ripgrep (rg)" }
func (t *searchFilesTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search pattern",
			},
		},
		"required": []string{"query"},
	}
}

func (t *searchFilesTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}

	if args.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	cmd := exec.CommandContext(ctx, "rg", "-n", "--color=never", args.Query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// rg returns exit code 1 when no matches, which is not an error for us.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return nil, fmt.Errorf("rg failed: %w\n%s", err, string(out))
	}

	return strings.TrimSpace(string(out)), nil
}

func NewSearchFilesTool() Tool { return &searchFilesTool{} }
