package projectpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	Init(wd)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"simple file", "src/main.go", false},
		{"nested", "docs/readme.md", false},
		{"escape", "../escape.go", true},
		{"absolute outside", "/etc/passwd", true},
		{"absolute inside", filepath.Join(wd, "go.mod"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abs, rel, err := Resolve(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Resolve(%q) should error", tt.path)
				}
				return
			}
			if err != nil {
				t.Errorf("Resolve(%q) unexpected error: %v", tt.path, err)
				return
			}
			if rel == "" {
				t.Error("expected non-empty relative path")
			}
			if abs == "" {
				t.Error("expected non-empty absolute path")
			}
		})
	}
}

func TestIsBlockedPath(t *testing.T) {
	blocked := []string{".env", ".git/config", "node_modules/foo", "dist/main.js", "bun.lock"}
	for _, p := range blocked {
		if !IsBlockedPath(p) {
			t.Errorf("IsBlockedPath(%q) should be true", p)
		}
	}
	allowed := []string{"src/main.go", "tests/foo_test.go", "README.md"}
	for _, p := range allowed {
		if IsBlockedPath(p) {
			t.Errorf("IsBlockedPath(%q) should be false", p)
		}
	}
}
