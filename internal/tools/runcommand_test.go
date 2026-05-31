package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/policy"
)

type countingPrompt struct {
	count  int
	result approval.Result
}

func (p *countingPrompt) Request(ctx context.Context, req approval.Request) (approval.Result, error) {
	p.count++
	return p.result, nil
}

func TestRunCommandRuntimePolicyPreApproval(t *testing.T) {
	runtimePolicy := policy.NewRuntimePolicy()
	runtimePolicy.AllowCommand("pwd -P")

	tool := NewRunCommandTool(execution.NewTracker(nil), &approval.NoOpPrompt{}, runtimePolicy)
	input, err := json.Marshal(map[string]any{"command": "pwd -P"})
	if err != nil {
		t.Fatal(err)
	}

	out, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := out.(map[string]any)
	if result["approvalRequired"] != false {
		t.Fatalf("approvalRequired = %v, want false", result["approvalRequired"])
	}
	if result["approved"] != true {
		t.Fatalf("approved = %v, want true", result["approved"])
	}
}

func TestRunCommandApprovalDecisions(t *testing.T) {
	t.Run("deny skips command", func(t *testing.T) {
		prompt := &countingPrompt{result: approval.Result{Decision: "deny", Reason: "no"}}
		tool := NewRunCommandTool(execution.NewTracker(nil), prompt, policy.NewRuntimePolicy())
		out, err := tool.Execute(context.Background(), mustCommandJSON(t, "pwd -P", "inspect cwd"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prompt.count != 1 {
			t.Fatalf("prompt count = %d, want 1", prompt.count)
		}
		result := out.(map[string]any)
		if result["skipped"] != true {
			t.Fatalf("skipped = %v, want true", result["skipped"])
		}
		if result["reason"] != "no" {
			t.Fatalf("reason = %v, want no", result["reason"])
		}
	})

	t.Run("approve once runs command", func(t *testing.T) {
		prompt := &countingPrompt{result: approval.Result{Decision: "approve_once"}}
		tool := NewRunCommandTool(execution.NewTracker(nil), prompt, policy.NewRuntimePolicy())
		out, err := tool.Execute(context.Background(), mustCommandJSON(t, "pwd -P", "inspect cwd"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prompt.count != 1 {
			t.Fatalf("prompt count = %d, want 1", prompt.count)
		}
		result := out.(map[string]any)
		if result["approved"] != true {
			t.Fatalf("approved = %v, want true", result["approved"])
		}
		if result["approvalRequired"] != true {
			t.Fatalf("approvalRequired = %v, want true", result["approvalRequired"])
		}
	})

	t.Run("always allow exact command skips later prompt", func(t *testing.T) {
		runtimePolicy := policy.NewRuntimePolicy()
		prompt := &countingPrompt{result: approval.Result{
			Decision: approval.DecisionAlwaysAllowCommand,
			PolicyAmendment: &approval.PolicyAmendment{
				Type:    "allow-command",
				Command: "pwd -P",
			},
		}}
		tool := NewRunCommandTool(execution.NewTracker(nil), prompt, runtimePolicy)
		if _, err := tool.Execute(context.Background(), mustCommandJSON(t, "pwd -P", "inspect cwd")); err != nil {
			t.Fatalf("first command failed: %v", err)
		}
		out, err := tool.Execute(context.Background(), mustCommandJSON(t, "pwd -P", ""))
		if err != nil {
			t.Fatalf("second command failed: %v", err)
		}
		if prompt.count != 1 {
			t.Fatalf("prompt count = %d, want 1", prompt.count)
		}
		result := out.(map[string]any)
		if result["approved"] != true {
			t.Fatalf("approved = %v, want true", result["approved"])
		}
		if result["approvalRequired"] != false {
			t.Fatalf("approvalRequired = %v, want false", result["approvalRequired"])
		}
	})

	t.Run("always allow does not cover different arguments", func(t *testing.T) {
		runtimePolicy := policy.NewRuntimePolicy()
		prompt := &countingPrompt{result: approval.Result{
			Decision: approval.DecisionAlwaysAllowCommand,
			PolicyAmendment: &approval.PolicyAmendment{
				Type:    "allow-command",
				Command: "pwd -P",
			},
		}}
		tool := NewRunCommandTool(execution.NewTracker(nil), prompt, runtimePolicy)
		if _, err := tool.Execute(context.Background(), mustCommandJSON(t, "pwd -P", "inspect cwd")); err != nil {
			t.Fatalf("first command failed: %v", err)
		}
		prompt.result = approval.Result{Decision: approval.DecisionDeny}
		if _, err := tool.Execute(context.Background(), mustCommandJSON(t, "pwd -L", "inspect logical cwd")); err != nil {
			t.Fatalf("second command failed: %v", err)
		}
		if prompt.count != 2 {
			t.Fatalf("prompt count = %d, want 2", prompt.count)
		}
	})
}

func TestRunCommandNonZeroExitIsFailure(t *testing.T) {
	tracker := execution.NewTracker(nil)
	prompt := &countingPrompt{result: approval.Result{Decision: approval.DecisionApproveOnce}}
	tool := NewRunCommandTool(tracker, prompt, policy.NewRuntimePolicy())

	_, err := tool.Execute(context.Background(), mustCommandJSON(t, "false", "verify failing command status"))
	if err == nil {
		t.Fatal("expected non-zero command to return an error")
	}

	records := tracker.ListRecords()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].Status != execution.StatusFailed {
		t.Fatalf("status = %q, want failed", records[0].Status)
	}
	if records[0].ExitCode == nil || *records[0].ExitCode != 1 {
		t.Fatalf("exitCode = %v, want 1", records[0].ExitCode)
	}
}

func mustCommandJSON(t *testing.T, command, reason string) []byte {
	t.Helper()
	input, err := json.Marshal(map[string]any{"command": command, "reason": reason})
	if err != nil {
		t.Fatal(err)
	}
	return input
}
