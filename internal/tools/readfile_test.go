package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

func TestReadFileTool_SmallFile(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	f := filepath.Join(dir, "hello.txt")
	os.WriteFile(f, []byte("line1\nline2\nline3\n"), 0644)

	tool := NewReadFileTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": "hello.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.(string), "line1") {
		t.Errorf("output missing line1: %q", out)
	}
}

func TestReadFileTool_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	f := filepath.Join(dir, "data.txt")
	os.WriteFile(f, []byte("a\nb\nc\nd\ne\n"), 0644)

	tool := NewReadFileTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":   "data.txt",
		"offset": 2,
		"limit":  2,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.(string)
	lines := strings.Split(got, "\n")
	// First line should be "b" (offset 2, line 2)
	if len(lines) < 2 || lines[0] != "b" || lines[1] != "c" {
		t.Errorf("expected b and c as first two lines, got: %q", got)
	}
}

func TestReadFileTool_TruncationByLines(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	f := filepath.Join(dir, "big.txt")
	lines := make([]string, maxReadLines+10)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i)
	}
	os.WriteFile(f, []byte(strings.Join(lines, "\n")), 0644)

	tool := NewReadFileTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": "big.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.(string)
	if !strings.Contains(got, "Use offset=") {
		t.Errorf("expected continuation hint, got: %q", got)
	}
	if strings.Contains(got, fmt.Sprintf("line%d", maxReadLines+5)) {
		t.Errorf("should not contain lines beyond limit")
	}
}

func TestReadFileTool_TruncationByBytes(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	f := filepath.Join(dir, "wide.txt")
	// Each line is 1000 chars, 60 lines = 60KB+ > 50KB limit.
	longLine := strings.Repeat("x", 1000)
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = longLine
	}
	os.WriteFile(f, []byte(strings.Join(lines, "\n")), 0644)

	tool := NewReadFileTool()
	out, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": "wide.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.(string)
	if !strings.Contains(got, "Use offset=") {
		t.Errorf("expected continuation hint, got: %q", got)
	}
}

func TestReadFileTool_OffsetBeyondEnd(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	f := filepath.Join(dir, "short.txt")
	os.WriteFile(f, []byte("a\nb\n"), 0644)

	tool := NewReadFileTool()
	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":   "short.txt",
		"offset": 10,
	}))
	if err == nil {
		t.Fatal("expected error for offset beyond end")
	}
}

func TestReadFileTool_Binary(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	f := filepath.Join(dir, "bin.dat")
	os.WriteFile(f, []byte{0x00, 0x01, 0x02}, 0644)

	tool := NewReadFileTool()
	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"path": "bin.dat"}))
	if err == nil {
		t.Fatal("expected error for binary file")
	}
}

func mustJSON(t *testing.T, v map[string]any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
