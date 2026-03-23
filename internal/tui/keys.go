package tui

import "charm.land/bubbles/v2/key"

// KeyMap defines all the keybindings for the application.
type KeyMap struct {
	// Global
	Quit key.Binding

	// Navigation
	Up     key.Binding
	Down   key.Binding
	Home   key.Binding
	End    key.Binding
	PageUp key.Binding
	PageDn key.Binding

	// Preview Pane
	PreviewUp   key.Binding
	PreviewDown key.Binding
	EnterChild  key.Binding

	// Actions
	Refresh    key.Binding
	Filter     key.Binding
	Assign     key.Binding
	Transition key.Binding
	Open       key.Binding
	Edit       key.Binding
	Comment    key.Binding
	Branch     key.Binding
	Extract    key.Binding
	New        key.Binding

	// Input Submission
	Submit key.Binding
	Cancel key.Binding
}

// DefaultKeyMap returns the standard ihj bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),

		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "ctrl+k"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "ctrl+j"),
		),
		Home: key.NewBinding(
			key.WithKeys("home"),
		),
		End: key.NewBinding(
			key.WithKeys("end"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
		),
		PageDn: key.NewBinding(
			key.WithKeys("pgdown"),
		),

		// Preview
		PreviewUp: key.NewBinding(
			key.WithKeys("shift+up", "ctrl+u"),
		),
		PreviewDown: key.NewBinding(
			key.WithKeys("shift+down", "ctrl+d"),
		),
		EnterChild: key.NewBinding(
			key.WithKeys("enter"),
		),

		// Actions
		Refresh: key.NewBinding(
			key.WithKeys("alt+r"),
			key.WithHelp("Alt-R", "Refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("alt+f"),
			key.WithHelp("Alt-F", "Filter"),
		),
		Assign: key.NewBinding(
			key.WithKeys("alt+a"),
			key.WithHelp("Alt-A", "Assign"),
		),
		Transition: key.NewBinding(
			key.WithKeys("alt+t"),
			key.WithHelp("Alt-T", "Transition"),
		),
		Open: key.NewBinding(
			key.WithKeys("alt+o"),
			key.WithHelp("Alt-O", "Open"),
		),
		Edit: key.NewBinding(
			key.WithKeys("alt+e"),
			key.WithHelp("Alt-E", "Edit"),
		),
		Comment: key.NewBinding(
			key.WithKeys("alt+c"),
			key.WithHelp("Alt-C", "Comment"),
		),
		Branch: key.NewBinding(
			key.WithKeys("alt+n"),
			key.WithHelp("Alt-N", "Branch"),
		),
		Extract: key.NewBinding(
			key.WithKeys("alt+x"),
			key.WithHelp("Alt-X", "Extract"),
		),
		New: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("Ctrl-N", "New"),
		),

		// Input
		Submit: key.NewBinding(
			key.WithKeys("alt+enter", "ctrl+s"),
			key.WithHelp("Alt+Enter", "Submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "Cancel"),
		),
	}
}
