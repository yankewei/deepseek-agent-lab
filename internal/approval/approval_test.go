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
