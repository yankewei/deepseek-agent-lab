package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

const maxToolOutputBytes = 50 * 1024

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		_, _ = b.buf.Write(p[:remaining])
	}
	if remaining < len(p) {
		b.truncated = true
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	out := b.buf.String()
	if b.truncated {
		out += fmt.Sprintf("\n\n[Output truncated at %d bytes.]", b.limit)
	}
	return out
}

type capturedCommand struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func runCapturedCommand(ctx context.Context, name string, args ...string) (capturedCommand, error) {
	stdout := newCappedBuffer(maxToolOutputBytes)
	stderr := newCappedBuffer(maxToolOutputBytes)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	result := capturedCommand{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}
	return result, err
}

type commandExecutionError struct {
	result capturedCommand
}

func (e *commandExecutionError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.result.ExitCode)
}

func (e *commandExecutionError) Details() any {
	return map[string]any{
		"stdout":   e.result.Stdout,
		"stderr":   e.result.Stderr,
		"exitCode": e.result.ExitCode,
	}
}

func (e *commandExecutionError) ExitCode() int {
	return e.result.ExitCode
}
