package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/projectpath"
)

func TestListFilesToolTruncatesLargeDirectories(t *testing.T) {
	dir := t.TempDir()
	projectpath.Init(dir)
	for i := 0; i < maxListEntries+1; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%04d.txt", i))
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	out, err := NewListFilesTool().Execute(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	entries := out.([]string)
	if len(entries) != maxListEntries+1 {
		t.Fatalf("len(entries) = %d, want %d", len(entries), maxListEntries+1)
	}
	if !strings.Contains(entries[len(entries)-1], "1 more entries") {
		t.Fatalf("last entry = %q, want truncation notice", entries[len(entries)-1])
	}
}
