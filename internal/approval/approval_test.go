package approval

import (
	"context"
	"testing"
)

func TestNoOpPrompt(t *testing.T) {
	p := &NoOpPrompt{}
	res, err := p.Request(context.Background(), Request{Action: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Decision != "deny" {
		t.Errorf("Decision = %q, want deny", res.Decision)
	}
}

func TestValidateResult(t *testing.T) {
	req := Request{
		SuggestedPolicyAmendment: &PolicyAmendment{
			Type:    "allow-command",
			Command: "go test ./...",
		},
	}

	t.Run("accept exact command amendment", func(t *testing.T) {
		err := ValidateResult(req, Result{
			Decision: DecisionAlwaysAllowCommand,
			PolicyAmendment: &PolicyAmendment{
				Type:    "allow-command",
				Command: "go test ./...",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("reject changed command amendment", func(t *testing.T) {
		err := ValidateResult(req, Result{
			Decision: DecisionAlwaysAllowCommand,
			PolicyAmendment: &PolicyAmendment{
				Type:    "allow-command",
				Command: "go test",
			},
		})
		if err == nil {
			t.Fatal("expected changed amendment to be rejected")
		}
	})

	t.Run("reject unknown decision", func(t *testing.T) {
		if err := ValidateResult(req, Result{Decision: "approve_everything"}); err == nil {
			t.Fatal("expected unknown decision to be rejected")
		}
	})
}
