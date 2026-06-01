package approvalform

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/tui/selector"
)

// New creates a huh form for an approval request using the generic selector.
// The decision is read via form.GetString("decision") after completion.
func New(req approval.Request) *huh.Form {
	opts := []selector.Choice{
		{Label: "Approve once", Value: "approve_once"},
		{Label: "Deny", Value: "deny"},
	}
	if req.SuggestedPolicyAmendment != nil {
		label := fmt.Sprintf("Always allow command: %s", req.SuggestedPolicyAmendment.Command)
		opts = append([]selector.Choice{
			{Label: label, Value: approval.DecisionAlwaysAllowCommand},
		}, opts...)
	}

	return selector.NewForm("⚠️  Approval Required", formatDetails(req), "decision", opts)
}

func formatDetails(req approval.Request) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Action: %s", req.Action))
	if req.Subject != "" {
		lines = append(lines, fmt.Sprintf("Subject: %s", req.Subject))
	}
	if req.RiskLevel != "" {
		lines = append(lines, fmt.Sprintf("Risk: %s", req.RiskLevel))
	}
	if req.PolicyReason != "" {
		lines = append(lines, fmt.Sprintf("Policy: %s", req.PolicyReason))
	}
	if len(req.Details) > 0 {
		lines = append(lines, "")
		for k, v := range req.Details {
			lines = append(lines, fmt.Sprintf("  %s: %s", k, v))
		}
	}
	return strings.Join(lines, "\n")
}
