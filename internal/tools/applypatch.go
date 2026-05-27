package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

type applyPatchTool struct{}

func (t *applyPatchTool) Name() string { return "applyPatch" }
func (t *applyPatchTool) Effect() Effect {
	return EffectWrite
}
func (t *applyPatchTool) Description() string {
	return "Apply a safe multi-file patch inside the current project"
}
func (t *applyPatchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patch": map[string]any{
				"type":        "string",
				"description": "The patch content",
			},
			"dryRun": map[string]any{
				"type":        "boolean",
				"description": "Preview changes without writing",
			},
		},
		"required": []string{"patch"},
	}
}

func (t *applyPatchTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Patch  string `json:"patch"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}

	ops, err := parsePatch(args.Patch)
	if err != nil {
		return nil, err
	}

	changed := make([]string, 0, len(ops))
	for _, op := range ops {
		abs, rel, err := projectpath.Resolve(op.Path)
		if err != nil {
			return nil, err
		}
		if projectpath.IsBlockedPath(rel) {
			return nil, fmt.Errorf("path is blocked: %s", rel)
		}

		changed = append(changed, rel)

		if args.DryRun {
			continue
		}

		switch op.Type {
		case "add":
			if err := os.WriteFile(abs, []byte(op.Content), 0644); err != nil {
				return nil, err
			}
		case "delete":
			if err := os.Remove(abs); err != nil {
				return nil, err
			}
		case "update":
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil, err
			}
			content := string(data)
			for _, hunk := range op.Hunks {
				count := strings.Count(content, hunk.OldText)
				if count == 0 {
					return nil, fmt.Errorf("patch hunk not found in %s", rel)
				}
				if count > 1 {
					return nil, fmt.Errorf("patch hunk appears %d times in %s", count, rel)
				}
				content = strings.Replace(content, hunk.OldText, hunk.NewText, 1)
			}
			if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
				return nil, err
			}
		}
	}

	return map[string]any{
		"changedFiles": changed,
		"dryRun":       args.DryRun,
	}, nil
}

func (t *applyPatchTool) ApprovalRequest(input json.RawMessage) (*approval.Request, bool, error) {
	var args struct {
		Patch  string `json:"patch"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, false, err
	}
	if args.DryRun {
		return nil, false, nil
	}
	ops, err := parsePatch(args.Patch)
	if err != nil {
		return nil, false, err
	}
	var deleted []string
	for _, op := range ops {
		if op.Type == "delete" {
			deleted = append(deleted, op.Path)
		}
	}
	if len(deleted) == 0 {
		return nil, false, nil
	}

	return &approval.Request{
		Action:       "apply-patch",
		Title:        "Delete file with applyPatch",
		Subject:      strings.Join(deleted, ", "),
		RiskLevel:    approval.RiskMedium,
		PolicyReason: "Patch deletes one or more files.",
		Details: map[string]string{
			"Deleted files": strings.Join(deleted, ", "),
		},
	}, true, nil
}

// patch types.
type patchOp struct {
	Type    string
	Path    string
	Content string
	Hunks   []hunk
}

type hunk struct {
	OldText string
	NewText string
}

func parsePatch(patch string) ([]patchOp, error) {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "*** Begin Patch" {
		return nil, fmt.Errorf("patch must start with *** Begin Patch")
	}
	if lines[len(lines)-1] != "*** End Patch" {
		return nil, fmt.Errorf("patch must end with *** End Patch")
	}

	var ops []patchOp
	i := 1
	for i < len(lines)-1 {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "*** Add File: "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "*** Add File: "))
			i++
			var content []string
			for i < len(lines)-1 && !isOpStart(lines[i]) {
				if !strings.HasPrefix(lines[i], "+") {
					return nil, fmt.Errorf("add file lines must start with +: %s", lines[i])
				}
				content = append(content, strings.TrimPrefix(lines[i], "+"))
				i++
			}
			if len(content) == 0 {
				return nil, fmt.Errorf("add file must have content: %s", path)
			}
			ops = append(ops, patchOp{Type: "add", Path: path, Content: strings.Join(content, "\n") + "\n"})

		case strings.HasPrefix(line, "*** Delete File: "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File: "))
			ops = append(ops, patchOp{Type: "delete", Path: path})
			i++

		case strings.HasPrefix(line, "*** Update File: "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "*** Update File: "))
			i++
			var hunks []hunk
			for i < len(lines)-1 && !isOpStart(lines[i]) {
				if lines[i] != "@@" {
					return nil, fmt.Errorf("update hunk must start with @@: %s", lines[i])
				}
				i++
				var oldLines, newLines []string
				for i < len(lines)-1 && lines[i] != "@@" && !isOpStart(lines[i]) {
					if len(lines[i]) == 0 {
						return nil, fmt.Errorf("empty hunk line")
					}
					switch lines[i][0] {
					case ' ':
						oldLines = append(oldLines, lines[i][1:])
						newLines = append(newLines, lines[i][1:])
					case '-':
						oldLines = append(oldLines, lines[i][1:])
					case '+':
						newLines = append(newLines, lines[i][1:])
					default:
						return nil, fmt.Errorf("hunk line must start with space, -, or +: %s", lines[i])
					}
					i++
				}
				if len(oldLines) == 0 && len(newLines) == 0 {
					return nil, fmt.Errorf("empty hunk")
				}
				hunks = append(hunks, hunk{
					OldText: strings.Join(oldLines, "\n") + "\n",
					NewText: strings.Join(newLines, "\n") + "\n",
				})
			}
			if len(hunks) == 0 {
				return nil, fmt.Errorf("update file must have hunks: %s", path)
			}
			ops = append(ops, patchOp{Type: "update", Path: path, Hunks: hunks})

		default:
			return nil, fmt.Errorf("unknown patch operation: %s", line)
		}
	}

	if len(ops) == 0 {
		return nil, fmt.Errorf("patch must have at least one operation")
	}
	return ops, nil
}

func isOpStart(line string) bool {
	return strings.HasPrefix(line, "*** Add File: ") ||
		strings.HasPrefix(line, "*** Delete File: ") ||
		strings.HasPrefix(line, "*** Update File: ")
}

func NewApplyPatchTool() Tool { return &applyPatchTool{} }
