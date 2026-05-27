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
	runtimePolicy.AllowPrefix("pwd")

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

	t.Run("always allow prefix skips later prompt", func(t *testing.T) {
		runtimePolicy := policy.NewRuntimePolicy()
		prompt := &countingPrompt{result: approval.Result{
			Decision: "always_allow_command_prefix",
			PolicyAmendment: &approval.PolicyAmendment{
				Type:   "allow-command-prefix",
				Prefix: "pwd -P",
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
}

func mustCommandJSON(t *testing.T, command, reason string) []byte {
	t.Helper()
	input, err := json.Marshal(map[string]any{"command": command, "reason": reason})
	if err != nil {
		t.Fatal(err)
	}
	return input
}
