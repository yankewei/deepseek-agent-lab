package projectpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	parent := t.TempDir()
	wd := filepath.Join(parent, "app")
	if err := os.Mkdir(wd, 0755); err != nil {
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
		{"dots in filename", "docs/v1..v2.md", false},
		{"escape", "../escape.go", true},
		{"absolute outside", "/etc/passwd", true},
		{"absolute inside", filepath.Join(wd, "go.mod"), false},
		{"absolute sibling with root prefix", filepath.Join(parent, "application", "go.mod"), true},
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

func TestResolveSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "secret-link")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	Init(root)
	if _, _, err := Resolve("secret-link"); err == nil {
		t.Fatal("expected symlink to file outside project to be rejected")
	}
}

func TestResolveNewWritableSymlinkParentEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "out-link")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	Init(root)
	if _, _, err := ResolveNewWritable("out-link/new.txt"); err == nil {
		t.Fatal("expected new file under symlinked outside directory to be rejected")
	}
}

func TestFindGitRoot(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	subdir := filepath.Join(repo, "internal", "tui")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: somewhere"), 0644); err != nil {
		t.Fatal(err)
	}

	got := FindGitRoot(subdir)
	if got != repo {
		t.Fatalf("FindGitRoot(%q) = %q, want %q", subdir, got, repo)
	}
}

func TestFindGitRootFallback(t *testing.T) {
	dir := t.TempDir()

	got := FindGitRoot(dir)
	if got != dir {
		t.Fatalf("FindGitRoot(%q) = %q, want %q (fallback to input)", dir, got, dir)
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
