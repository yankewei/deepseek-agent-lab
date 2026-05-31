package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

type listFilesTool struct{}

const maxListEntries = 2000

func (t *listFilesTool) Name() string        { return "listFiles" }
func (t *listFilesTool) Effect() Effect      { return EffectRead }
func (t *listFilesTool) Description() string { return "List files inside the current project" }
func (t *listFilesTool) Schema() map[string]any {
	return objectSchema(
		map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative directory path to list (default: root)",
			},
		},
	)
}

func (t *listFilesTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := decodeInput(input, &args); err != nil {
		return nil, err
	}

	abs, rel, err := projectpath.Resolve(args.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", rel)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	var files, dirs []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)

	listed := append(dirs, files...)
	if len(listed) > maxListEntries {
		remaining := len(listed) - maxListEntries
		listed = append(listed[:maxListEntries], fmt.Sprintf("[... %d more entries]", remaining))
	}
	return listed, nil
}

func NewListFilesTool() Tool { return &listFilesTool{} }
