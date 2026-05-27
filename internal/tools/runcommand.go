package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/policy"
)

type runCommandTool struct {
	tracker       *execution.Tracker
	prompt        approval.Prompt
	runtimePolicy *policy.RuntimePolicy
}

func (t *runCommandTool) Name() string { return "runCommand" }
func (t *runCommandTool) Description() string {
	return "Run a project command allowed by policy, asking for approval when required"
}
func (t *runCommandTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Command to run",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Reason for running this command (required for approval)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *runCommandTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Command string `json:"command"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}

	rec := t.tracker.CreateRecord("command", "", args.Command, args.Reason)

	decision := policy.Evaluate(args.Command)
	t.tracker.UpdateRecord(rec.ID, map[string]any{
		"status":             execution.StatusPolicyEvaluated,
		"policy_decision":    decision.Type,
		"policy_code":        string(decision.Code),
		"policy_reason":      decision.Reason,
		"normalized_command": decision.Command,
	})

	if decision.Type == "forbidden" {
		t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": decision.Reason})
		return nil, fmt.Errorf("%s", decision.Reason)
	}

	approvalRequired := decision.Type == "prompt"
	approved := false

	if approvalRequired && !t.runtimePolicy.IsAllowed(args.Command) {
		if args.Reason == "" {
			err := fmt.Errorf("approval reason is required for command: %s", args.Command)
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			return nil, err
		}

		t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusWaitingApproval})

		var amend *approval.PolicyAmendment
		if prefix := policy.GetApprovablePrefix(args.Command); prefix != "" {
			amend = &approval.PolicyAmendment{Type: "allow-command-prefix", Prefix: prefix}
		}

		res, err := t.prompt.Request(ctx, approval.Request{
			Action:                   "run-command",
			Title:                    "Run command requiring approval",
			Subject:                  args.Command,
			RiskLevel:                approval.RiskLevel(decision.RiskLevel),
			PolicyReason:             decision.Reason,
			SuggestedPolicyAmendment: amend,
			Details:                  map[string]string{"Command": args.Command, "Reason": args.Reason},
		})
		if err != nil {
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			return nil, err
		}

		if res.Decision == "deny" {
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusDenied, "error": res.Reason})
			return map[string]any{"skipped": true, "approvalRequired": true}, nil
		}

		approved = true
		if res.Decision == "always_allow_command_prefix" && res.PolicyAmendment != nil {
			t.runtimePolicy.AllowPrefix(res.PolicyAmendment.Prefix)
		}
	} else if approvalRequired {
		approvalRequired = false
	}

	t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusRunning})

	parts := splitCommand(decision.Command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			return nil, err
		}
	}

	t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusCompleted, "exit_code": exitCode})

	return map[string]any{
		"stdout":           string(out),
		"stderr":           "",
		"exitCode":         exitCode,
		"approved":         approved,
		"approvalRequired": approvalRequired,
	}, nil
}

func splitCommand(cmd string) []string {
	return splitWhitespace(cmd)
}

func splitWhitespace(s string) []string {
	var parts []string
	for _, f := range splitFields(s) {
		if f != "" {
			parts = append(parts, f)
		}
	}
	return parts
}

func splitFields(s string) []string {
	return splitCommandInternal(s)
}

func splitCommandInternal(s string) []string {
	// Simple whitespace split; sufficient for our allowed command set.
	var result []string
	var current []rune
	inQuote := false
	quoteChar := rune(0)
	for _, r := range s {
		switch {
		case r == ' ' || r == '\t':
			if inQuote {
				current = append(current, r)
			} else if len(current) > 0 {
				result = append(result, string(current))
				current = nil
			}
		case r == '"' || r == '\'':
			if inQuote && r == quoteChar {
				inQuote = false
				quoteChar = 0
			} else if !inQuote {
				inQuote = true
				quoteChar = r
			} else {
				current = append(current, r)
			}
		default:
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

// NewRunCommandTool creates the runCommand tool.
func NewRunCommandTool(tracker *execution.Tracker, prompt approval.Prompt, runtimePolicy *policy.RuntimePolicy) Tool {
	return &runCommandTool{tracker: tracker, prompt: prompt, runtimePolicy: runtimePolicy}
}
