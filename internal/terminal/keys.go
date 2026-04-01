package terminal

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

	// Vim mode switches
	Search  key.Binding
	Command key.Binding

	// Input Submission
	Submit key.Binding
	Cancel key.Binding
}

// ActionBindings returns the action key bindings in display order.
func (k KeyMap) ActionBindings() []key.Binding {
	return []key.Binding{
		k.Refresh, k.Filter, k.Assign, k.Transition,
		k.Open, k.Edit, k.Comment, k.Branch,
		k.Extract, k.New,
	}
}

// VimKeyMap returns bindings for vim mode: single-char action keys and
// j/k/g/G navigation. The help text reflects the vim-style keys.
func VimKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),

		// Navigation — vim j/k plus arrows.
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
		),
		Home: key.NewBinding(
			key.WithKeys("g", "home"),
		),
		End: key.NewBinding(
			key.WithKeys("G", "end"),
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

		// Actions — single-char keys.
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "Refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "Filter"),
		),
		Assign: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "Assign"),
		),
		Transition: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "Transition"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "Open"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "Edit"),
		),
		Comment: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "Comment"),
		),
		Branch: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "Branch"),
		),
		Extract: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "Extract"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "New"),
		),

		// Vim mode switches
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "Search"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "Command"),
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
			key.WithKeys("alt+f", "ctrl+f"),
			key.WithHelp("Alt-F", "Filter"),
		),
		Assign: key.NewBinding(
			key.WithKeys("alt+a"),
			key.WithHelp("Alt-A", "Assign"),
		),
		Transition: key.NewBinding(
			key.WithKeys("alt+t", "ctrl+t"),
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
			key.WithKeys("alt+c", "ctrl+k"),
			key.WithHelp("Alt-C", "Comment"),
		),
		Branch: key.NewBinding(
			key.WithKeys("alt+n"),
			key.WithHelp("Alt-N", "Branch"),
		),
		Extract: key.NewBinding(
			key.WithKeys("alt+x", "ctrl+x"),
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
