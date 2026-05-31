package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

func TestApplyPatchTool(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)

	// Create a test file for update.
	existingFile := filepath.Join(dir, "existing.go")
	os.WriteFile(existingFile, []byte("package main\n\nfunc main() {}\n"), 0644)

	tool := NewApplyPatchTool()

	t.Run("add file", func(t *testing.T) {
		patch := "*** Begin Patch\n*** Add File: new.go\n+package new\n*** End Patch"
		input, _ := json.Marshal(map[string]any{"patch": patch})
		res, err := tool.Execute(nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result := res.(map[string]any)
		if result["dryRun"].(bool) {
			t.Fatal("expected dryRun false")
		}
		if _, err := os.Stat(filepath.Join(dir, "new.go")); err != nil {
			t.Fatal("new.go should exist")
		}
	})

	t.Run("update file", func(t *testing.T) {
		patch := "*** Begin Patch\n*** Update File: existing.go\n@@\n-func main() {}\n+func main() { println(\"hi\") }\n*** End Patch"
		input, _ := json.Marshal(map[string]any{"patch": patch})
		_, err := tool.Execute(nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content, _ := os.ReadFile(existingFile)
		expected := "package main\n\nfunc main() { println(\"hi\") }\n"
		if string(content) != expected {
			t.Fatalf("unexpected content: %q", string(content))
		}
	})

	t.Run("delete file requires approval", func(t *testing.T) {
		patch := "*** Begin Patch\n*** Delete File: new.go\n*** End Patch"
		input, _ := json.Marshal(map[string]any{"patch": patch})
		_, required, err := tool.(*applyPatchTool).ApprovalRequest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !required {
			t.Fatal("delete patch should require approval")
		}
	})

	t.Run("delete dry run does not require approval", func(t *testing.T) {
		patch := "*** Begin Patch\n*** Delete File: new.go\n*** End Patch"
		input, _ := json.Marshal(map[string]any{"patch": patch, "dryRun": true})
		_, required, err := tool.(*applyPatchTool).ApprovalRequest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if required {
			t.Fatal("delete patch dry run should not require approval")
		}
	})

	t.Run("dry run", func(t *testing.T) {
		patch := "*** Begin Patch\n*** Add File: dry.go\n+package dry\n*** End Patch"
		input, _ := json.Marshal(map[string]any{"patch": patch, "dryRun": true})
		res, err := tool.Execute(nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result := res.(map[string]any)
		if !result["dryRun"].(bool) {
			t.Fatal("expected dryRun true")
		}
		if _, err := os.Stat(filepath.Join(dir, "dry.go")); err == nil {
			t.Fatal("dry.go should not exist in dry run")
		}
	})

	t.Run("invalid patch format", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{"patch": "invalid"})
		_, err := tool.Execute(nil, input)
		if err == nil {
			t.Fatal("expected error for invalid patch")
		}
	})
}

func TestApplyPatchDryRunValidatesUpdateHunks(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewApplyPatchTool()
	patch := "*** Begin Patch\n*** Update File: existing.go\n@@\n-missing\n+replacement\n*** End Patch"
	input, _ := json.Marshal(map[string]any{"patch": patch, "dryRun": true})

	if _, err := tool.Execute(nil, input); err == nil {
		t.Fatal("dry-run should reject a missing update hunk")
	}
}

func TestApplyPatchValidatesAllOperationsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	first := filepath.Join(dir, "first.go")
	second := filepath.Join(dir, "second.go")
	if err := os.WriteFile(first, []byte("package first\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("package second\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewApplyPatchTool()
	patch := "*** Begin Patch\n*** Update File: first.go\n@@\n-package first\n+package changed\n*** Update File: second.go\n@@\n-package missing\n+package changed\n*** End Patch"
	input, _ := json.Marshal(map[string]any{"patch": patch})

	if _, err := tool.Execute(nil, input); err == nil {
		t.Fatal("patch should reject the invalid second operation")
	}
	content, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package first\n" {
		t.Fatalf("first file changed before full validation: %q", content)
	}
}

func TestApplyPatchAddDoesNotOverwriteExistingFile(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	target := filepath.Join(dir, "existing.go")
	if err := os.WriteFile(target, []byte("package original\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewApplyPatchTool()
	patch := "*** Begin Patch\n*** Add File: existing.go\n+package replacement\n*** End Patch"
	input, _ := json.Marshal(map[string]any{"patch": patch})

	if _, err := tool.Execute(nil, input); err == nil {
		t.Fatal("add should reject an existing file")
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package original\n" {
		t.Fatalf("existing file was overwritten: %q", content)
	}
}
