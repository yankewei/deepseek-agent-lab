package approvalform

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/yankewei/ds-coding-agent/internal/approval"
)

// New creates a huh form for an approval request.
// It returns the form and a pointer to the decision variable that will be
// populated when the form is completed.
func New(req approval.Request) (*huh.Form, *string) {
	var decision string

	opts := []huh.Option[string]{
		huh.NewOption("Approve once", "approve_once"),
		huh.NewOption("Deny", "deny"),
	}
	if req.SuggestedPolicyAmendment != nil {
		label := fmt.Sprintf("Always allow prefix: %s", req.SuggestedPolicyAmendment.Prefix)
		opts = append([]huh.Option[string]{
			huh.NewOption(label, "always_allow_command_prefix"),
		}, opts...)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("⚠️  Approval Required").
				Description(formatDetails(req)),
			huh.NewSelect[string]().
				Key("decision").
				Title("Choose action").
				Options(opts...).
				Value(&decision),
		),
	)

	return form, &decision
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
