package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

const (
	maxReadLines = 2000
	maxReadBytes = 50 * 1024 // 50 KB
)

type readFileTool struct{}

func (t *readFileTool) Name() string        { return "readFile" }
func (t *readFileTool) Description() string { return "Read a project file. Large files are truncated to 2000 lines or 50KB. Use offset/limit to read specific ranges." }
func (t *readFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative path to the file",
			},
			"offset": map[string]any{
				"type":        "number",
				"description": "Line number to start reading from (1-indexed)",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "Maximum number of lines to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *readFileTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}

	abs, _, err := projectpath.Resolve(args.Path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	if !utf8.Valid(data) || strings.Contains(string(data), "\x00") {
		return nil, fmt.Errorf("file appears to be binary: %s", args.Path)
	}

	allLines := strings.Split(string(data), "\n")
	totalLines := len(allLines)

	// Apply offset (1-indexed input -> 0-indexed array).
	startLine := 0
	if args.Offset > 0 {
		startLine = args.Offset - 1
	}
	if startLine >= len(allLines) {
		return nil, fmt.Errorf("offset %d is beyond end of file (%d lines)", args.Offset, totalLines)
	}

	// Apply user limit if specified.
	endLine := len(allLines)
	if args.Limit > 0 {
		endLine = startLine + args.Limit
		if endLine > len(allLines) {
			endLine = len(allLines)
		}
	}

	selected := allLines[startLine:endLine]

	// Apply default truncation (lines + bytes).
	outputLines, truncated := truncateLines(selected)

	result := strings.Join(outputLines, "\n")

	// Build continuation notice if truncated or user limit stopped early.
	if truncated || (args.Limit > 0 && endLine < len(allLines)) {
		outputLineCount := len(outputLines)
		endLineDisplay := startLine + outputLineCount
		nextOffset := endLineDisplay + 1

		if truncated {
			result += fmt.Sprintf(
				"\n\n[Showing lines %d-%d of %d. Use offset=%d to continue.]",
				startLine+1, endLineDisplay, totalLines, nextOffset,
			)
		} else {
			remaining := len(allLines) - endLine
			result += fmt.Sprintf(
				"\n\n[%d more lines in file. Use offset=%d to continue.]",
				remaining, nextOffset,
			)
		}
	}

	return result, nil
}

// truncateLines truncates to maxReadLines and maxReadBytes.
// It never returns partial lines.
func truncateLines(lines []string) ([]string, bool) {
	if len(lines) <= maxReadLines {
		byteCount := 0
		for i, line := range lines {
			lineBytes := len(line)
			if i > 0 {
				lineBytes++ // newline
			}
			if byteCount+lineBytes > maxReadBytes {
				return lines[:i], true
			}
			byteCount += lineBytes
		}
		return lines, false
	}

	// Exceeds line limit; check byte limit on the first maxReadLines.
	byteCount := 0
	for i := 0; i < maxReadLines; i++ {
		lineBytes := len(lines[i])
		if i > 0 {
			lineBytes++
		}
		if byteCount+lineBytes > maxReadBytes {
			return lines[:i], true
		}
		byteCount += lineBytes
	}
	return lines[:maxReadLines], true
}

func NewReadFileTool() Tool { return &readFileTool{} }
