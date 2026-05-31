package selector

import (
	"testing"

	"charm.land/huh/v2"
)

func TestNewFormCreatesFormWithChoices(t *testing.T) {
	choices := []Choice{
		{Label: "A", Value: "a"},
		{Label: "B", Value: "b"},
	}
	form := NewForm("Title", "Description", "key", choices)
	if form == nil {
		t.Fatal("NewForm returned nil")
	}
	if form.State != huh.StateNormal {
		t.Fatalf("form.State = %v, want %v", form.State, huh.StateNormal)
	}
}

func TestNewFormWithEmptyChoices(t *testing.T) {
	form := NewForm("Title", "Description", "key", nil)
	if form == nil {
		t.Fatal("NewForm with nil choices returned nil")
	}
}
