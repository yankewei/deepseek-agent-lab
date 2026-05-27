package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

func TestEditFileTool(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)

	// Create a test file.
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\nfoo bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewEditFileTool()

	t.Run("successful replacement", func(t *testing.T) {
		input := []byte(`{"path": "test.txt", "oldText": "hello world\n", "newText": "hi universe\n"}`)
		res, err := tool.Execute(nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = res

		content, _ := os.ReadFile(testFile)
		if string(content) != "hi universe\nfoo bar\n" {
			t.Fatalf("unexpected content: %q", string(content))
		}
	})

	t.Run("oldText not found", func(t *testing.T) {
		input := []byte(`{"path": "test.txt", "oldText": "nonexistent", "newText": "x"}`)
		_, err := tool.Execute(nil, input)
		if err == nil {
			t.Fatal("expected error for missing oldText")
		}
	})

	t.Run("ambiguous oldText", func(t *testing.T) {
		// Write file with duplicate line.
		os.WriteFile(testFile, []byte("dup\ndup\n"), 0644)
		input := []byte(`{"path": "test.txt", "oldText": "dup\n", "newText": "x"}`)
		_, err := tool.Execute(nil, input)
		if err == nil {
			t.Fatal("expected error for ambiguous oldText")
		}
	})

	t.Run("blocked path", func(t *testing.T) {
		input := []byte(`{"path": ".env", "oldText": "x", "newText": "y"}`)
		_, err := tool.Execute(nil, input)
		if err == nil {
			t.Fatal("expected error for blocked path")
		}
	})
}
