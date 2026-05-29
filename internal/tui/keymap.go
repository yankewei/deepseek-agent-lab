package tui

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines the key bindings for the TUI.
type KeyMap struct {
	Submit      key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Quit        key.Binding
	Cancel      key.Binding
	Search      key.Binding
	Help        key.Binding
	Regenerate  key.Binding
	CopyMessage key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Submit:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "send")),
		PageUp:      key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
		PageDown:    key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "scroll down")),
		Quit:        key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Cancel:      key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "cancel")),
		Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Regenerate:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "regenerate")),
		CopyMessage: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
	}
}

// ShortHelp returns the short help bindings.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Search, k.Help, k.Quit}
}

// FullHelp returns the full help bindings.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Submit, k.PageUp, k.PageDown, k.Quit},
		{k.Cancel, k.Search, k.Help, k.Regenerate, k.CopyMessage},
	}
}
