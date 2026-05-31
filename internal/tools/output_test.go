package tools

import (
	"context"
	"strings"
	"testing"
)

func TestCappedBufferTruncatesOutput(t *testing.T) {
	buf := newCappedBuffer(4)
	if _, err := buf.Write([]byte("abcdef")); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.HasPrefix(got, "abcd") {
		t.Fatalf("output = %q, want captured prefix", got)
	}
	if !strings.Contains(got, "Output truncated") {
		t.Fatalf("output = %q, want truncation notice", got)
	}
}

func TestRunCapturedCommandSeparatesStdoutAndStderr(t *testing.T) {
	result, err := runCapturedCommand(context.Background(), "sh", "-c", "printf out; printf err >&2")
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "out" {
		t.Fatalf("stdout = %q, want out", result.Stdout)
	}
	if result.Stderr != "err" {
		t.Fatalf("stderr = %q, want err", result.Stderr)
	}
}
