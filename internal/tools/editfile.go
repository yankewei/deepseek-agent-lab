package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

type editFileTool struct{}

func (t *editFileTool) Name() string        { return "editFile" }
func (t *editFileTool) Effect() Effect      { return EffectWrite }
func (t *editFileTool) Description() string { return "Replace one exact text block in one file" }
func (t *editFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative path to the file",
			},
			"oldText": map[string]any{
				"type":        "string",
				"description": "Exact text to replace (must appear exactly once)",
			},
			"newText": map[string]any{
				"type":        "string",
				"description": "Replacement text",
			},
		},
		"required": []string{"path", "oldText", "newText"},
	}
}

func (t *editFileTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Path    string `json:"path"`
		OldText string `json:"oldText"`
		NewText string `json:"newText"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}

	abs, rel, err := projectpath.Resolve(args.Path)
	if err != nil {
		return nil, err
	}

	if projectpath.IsBlockedPath(rel) {
		return nil, fmt.Errorf("path is blocked: %s", rel)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	content := string(data)
	count := strings.Count(content, args.OldText)
	if count == 0 {
		return nil, fmt.Errorf("oldText not found in %s", rel)
	}
	if count > 1 {
		return nil, fmt.Errorf("oldText appears %d times in %s; provide more context", count, rel)
	}

	newContent := strings.Replace(content, args.OldText, args.NewText, 1)
	if err := os.WriteFile(abs, []byte(newContent), 0644); err != nil {
		return nil, err
	}

	return map[string]string{"path": rel}, nil
}

func NewEditFileTool() Tool { return &editFileTool{} }
