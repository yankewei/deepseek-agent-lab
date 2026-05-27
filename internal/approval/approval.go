package approval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// RiskLevel mirrors policy.RiskLevel for independence.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Request is presented to the user when approval is needed.
type Request struct {
	Action                   string            `json:"action"`
	Title                    string            `json:"title"`
	Subject                  string            `json:"subject"`
	RiskLevel                RiskLevel         `json:"risk_level"`
	PolicyReason             string            `json:"policy_reason"`
	SuggestedPolicyAmendment *PolicyAmendment  `json:"suggested_policy_amendment,omitempty"`
	Details                  map[string]string `json:"details"`
}

// PolicyAmendment suggests a runtime policy change.
type PolicyAmendment struct {
	Type   string `json:"type"`
	Prefix string `json:"prefix"`
}

// Result is the user's decision.
type Result struct {
	Decision        string           `json:"decision"` // "approve_once", "always_allow_command_prefix", "deny"
	Reason          string           `json:"reason,omitempty"`
	PolicyAmendment *PolicyAmendment `json:"policy_amendment,omitempty"`
}

// Prompt is the interface for requesting approval.
type Prompt interface {
	Request(ctx context.Context, req Request) (Result, error)
}

// StdinPrompt is a CLI-based approval prompt.
type StdinPrompt struct{}

// Request prints the approval request to stderr and reads from stdin.
func (s *StdinPrompt) Request(ctx context.Context, req Request) (Result, error) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "--- Approval Required ---")
	fmt.Fprintf(os.Stderr, "Action: %s\n", req.Action)
	if req.Subject != "" {
		fmt.Fprintf(os.Stderr, "Subject: %s\n", req.Subject)
	}
	if req.RiskLevel != "" {
		fmt.Fprintf(os.Stderr, "Risk: %s\n", req.RiskLevel)
	}
	if req.PolicyReason != "" {
		fmt.Fprintf(os.Stderr, "Policy: %s\n", req.PolicyReason)
	}
	if len(req.Details) > 0 {
		fmt.Fprintln(os.Stderr, "Details:")
		for k, v := range req.Details {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", k, v)
		}
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprint(os.Stderr, "Approve? [y=once/a=always/n=deny]: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return Result{Decision: "deny", Reason: fmt.Sprintf("Read error: %v", err)}, nil
	}
	line = strings.TrimSpace(strings.ToLower(line))

	switch line {
	case "y":
		return Result{Decision: "approve_once"}, nil
	case "a":
		var amend *PolicyAmendment
		if req.SuggestedPolicyAmendment != nil {
			amend = req.SuggestedPolicyAmendment
		}
		return Result{Decision: "always_allow_command_prefix", PolicyAmendment: amend}, nil
	case "n":
		return Result{Decision: "deny", Reason: "Denied by user."}, nil
	default:
		return Result{Decision: "deny", Reason: fmt.Sprintf("Unrecognized response '%s'.", line)}, nil
	}
}

// NoOpPrompt always denies. Useful for non-interactive tests.
type NoOpPrompt struct{}

func (n *NoOpPrompt) Request(ctx context.Context, req Request) (Result, error) {
	return Result{Decision: "deny", Reason: "No approval prompt configured."}, nil
}
