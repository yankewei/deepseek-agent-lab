package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

const maxFileSize = 100 * 1024 // 100 KB

type readFileTool struct{}

func (t *readFileTool) Name() string        { return "readFile" }
func (t *readFileTool) Description() string { return "Read a project file (max 100 KB, text only)" }
func (t *readFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative path to the file",
			},
		},
		"required": []string{"path"},
	}
}

func (t *readFileTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}

	abs, _, err := projectpath.Resolve(args.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}

	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file too large: %s is %d bytes (max %d)", args.Path, info.Size(), maxFileSize)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	if !utf8.Valid(data) {
		return nil, fmt.Errorf("file appears to be binary: %s", args.Path)
	}

	return string(data), nil
}

func NewReadFileTool() Tool { return &readFileTool{} }
