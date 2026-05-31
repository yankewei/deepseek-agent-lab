package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/policy"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
)

type runCommandTool struct {
	tracker       *execution.Tracker
	prompt        approval.Prompt
	runtimePolicy *policy.RuntimePolicy
	logger        *runlog.Logger
}

func (t *runCommandTool) Name() string { return "runCommand" }
func (t *runCommandTool) Effect() Effect {
	return EffectCommand
}
func (t *runCommandTool) Description() string {
	return "Run a project command allowed by policy, asking for approval when required"
}
func (t *runCommandTool) SetApprovalPrompt(prompt approval.Prompt) {
	t.prompt = prompt
}
func (t *runCommandTool) Schema() map[string]any {
	return objectSchema(
		map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Command to run",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Reason for running this command (required for approval)",
			},
		},
		"command",
	)
}

func (t *runCommandTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var args struct {
		Command string `json:"command"`
		Reason  string `json:"reason"`
	}
	if err := decodeInput(input, &args); err != nil {
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
	preApproved := approvalRequired && t.runtimePolicy.IsAllowed(args.Command)

	if approvalRequired && !preApproved {
		if args.Reason == "" {
			err := fmt.Errorf("approval reason is required for command: %s", args.Command)
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			return nil, err
		}

		t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusWaitingApproval})

		var amend *approval.PolicyAmendment
		if command := policy.GetApprovableCommand(args.Command); command != "" {
			amend = &approval.PolicyAmendment{Type: "allow-command", Command: command}
		}

		req := approval.Request{
			Action:                   "run-command",
			Title:                    "Run command requiring approval",
			Subject:                  args.Command,
			RiskLevel:                approval.RiskLevel(decision.RiskLevel),
			PolicyReason:             decision.Reason,
			SuggestedPolicyAmendment: amend,
			Details:                  map[string]string{"Command": args.Command, "Reason": args.Reason},
		}
		approvalID := ""
		if t.logger != nil {
			id, _ := t.logger.AppendApprovalRequested(req, rec.ID)
			approvalID = id
		}
		prompt := t.prompt
		if prompt == nil {
			prompt = &approval.NoOpPrompt{}
		}
		res, err := prompt.Request(ctx, req)
		if err != nil {
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			return nil, err
		}
		if t.logger != nil {
			_ = t.logger.AppendApprovalResolved(approvalID, res, rec.ID)
		}
		if err := approval.ValidateResult(req, res); err != nil {
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			return nil, err
		}

		if res.Decision == approval.DecisionDeny {
			reason := res.Reason
			if reason == "" {
				reason = "Denied by user."
			}
			t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusDenied, "error": reason})
			return map[string]any{"skipped": true, "approvalRequired": true, "reason": reason}, nil
		}

		approved = true
		if res.Decision == approval.DecisionAlwaysAllowCommand && res.PolicyAmendment != nil {
			t.runtimePolicy.AllowCommand(res.PolicyAmendment.Command)
		}
	} else if preApproved {
		approvalRequired = false
		approved = true
	}

	t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusRunning})

	parts, err := splitCommand(decision.Command)
	if err != nil {
		t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
		return nil, err
	}
	result, err := runCapturedCommand(ctx, parts[0], parts[1:]...)
	if err != nil {
		t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "exit_code": result.ExitCode, "error": err.Error()})
		if result.ExitCode != 0 {
			return nil, &commandExecutionError{result: result}
		}
		return nil, err
	}

	t.tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusCompleted, "exit_code": result.ExitCode})

	return map[string]any{
		"stdout":           result.Stdout,
		"stderr":           result.Stderr,
		"exitCode":         result.ExitCode,
		"approved":         approved,
		"approvalRequired": approvalRequired,
	}, nil
}

func splitCommand(cmd string) ([]string, error) {
	// Simple whitespace split; sufficient for our allowed command set.
	var result []string
	var current []rune
	inQuote := false
	quoteChar := rune(0)
	for _, r := range cmd {
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
	if inQuote {
		return nil, fmt.Errorf("command contains an unclosed quote")
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}
	return result, nil
}

// NewRunCommandTool creates the runCommand tool.
func NewRunCommandTool(tracker *execution.Tracker, prompt approval.Prompt, runtimePolicy *policy.RuntimePolicy) Tool {
	return NewRunCommandToolWithLogger(tracker, prompt, runtimePolicy, nil)
}

// NewRunCommandToolWithLogger creates the runCommand tool with optional run logging.
func NewRunCommandToolWithLogger(tracker *execution.Tracker, prompt approval.Prompt, runtimePolicy *policy.RuntimePolicy, logger *runlog.Logger) Tool {
	return &runCommandTool{tracker: tracker, prompt: prompt, runtimePolicy: runtimePolicy, logger: logger}
}
