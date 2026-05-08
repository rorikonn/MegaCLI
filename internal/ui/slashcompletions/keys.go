package slashcompletions

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines the key bindings for the slash completions component.
type KeyMap struct {
	Down,
	Up,
	Select,
	Cancel key.Binding
}

// DefaultKeyMap returns the default key bindings for slash completions.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("down", "move down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("up", "move up"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "tab"),
			key.WithHelp("tab", "select"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}
