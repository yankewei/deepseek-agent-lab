// Package selector provides a generic single-choice form built on huh.
package selector

import (
	"charm.land/huh/v2"
)

// Choice describes one selectable option.
type Choice struct {
	Label string
	Value string
}

// NewForm creates a huh.Form with a note and a single select field.
// The select field uses the given key. The result is read via
// form.GetString(selectKey) after the form reaches StateCompleted.
func NewForm(noteTitle, noteDescription, selectKey string, choices []Choice) *huh.Form {
	var selected string

	opts := make([]huh.Option[string], len(choices))
	for i, c := range choices {
		opts[i] = huh.NewOption(c.Label, c.Value)
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(noteTitle).
				Description(noteDescription),
			huh.NewSelect[string]().
				Key(selectKey).
				Title("Choose").
				Options(opts...).
				Value(&selected),
		),
	)
}
