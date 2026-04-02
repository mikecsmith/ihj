package terminal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
)

// KeyMap defines all the keybindings for the application.
type KeyMap struct {
	// Global
	Quit key.Binding
	Help key.Binding

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
	Workspace  key.Binding

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
		k.Extract, k.New, k.Workspace,
	}
}

// ShortHelp returns the bindings for a single-line help view.
// Implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	bindings := k.ActionBindings()
	// Include search binding if it has help text (vim mode).
	if k.Search.Help().Key != "" {
		bindings = append(bindings, k.Search)
	}
	return bindings
}

// FullHelp returns grouped bindings for a multi-column help view.
// Implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Home, k.End, k.PageUp, k.PageDn},
		{k.PreviewUp, k.PreviewDown, k.EnterChild},
		{k.Refresh, k.Filter, k.Assign, k.Transition, k.Open},
		{k.Edit, k.Comment, k.Branch, k.Extract, k.New, k.Workspace},
		{k.Cancel, k.Quit},
	}
}

// VimKeyMap returns bindings for vim mode: single-char action keys and
// j/k/g/G navigation. The help text reflects the vim-style keys.
func VimKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "Quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "Help"),
		),

		// Navigation — vim j/k plus arrows.
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "Up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "Down"),
		),
		Home: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "Top"),
		),
		End: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "Bottom"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("PgUp", "Page Up"),
		),
		PageDn: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("PgDn", "Page Down"),
		),

		// Preview
		PreviewUp: key.NewBinding(
			key.WithKeys("shift+up", "ctrl+u"),
			key.WithHelp("C-u", "Preview Up"),
		),
		PreviewDown: key.NewBinding(
			key.WithKeys("shift+down", "ctrl+d"),
			key.WithHelp("C-d", "Preview Down"),
		),
		EnterChild: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "Open Child"),
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
		Workspace: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "Workspace"),
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
			key.WithHelp("Ctrl-C", "Quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("alt+/"),
			key.WithHelp("Alt-/", "Help"),
		),

		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "ctrl+k"),
			key.WithHelp("↑/Ctrl-K", "Up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "ctrl+j"),
			key.WithHelp("↓/Ctrl-J", "Down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("Home", "Top"),
		),
		End: key.NewBinding(
			key.WithKeys("end"),
			key.WithHelp("End", "Bottom"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("PgUp", "Page Up"),
		),
		PageDn: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("PgDn", "Page Down"),
		),

		// Preview
		PreviewUp: key.NewBinding(
			key.WithKeys("shift+up", "ctrl+u"),
			key.WithHelp("Shift-↑/Ctrl-U", "Preview Up"),
		),
		PreviewDown: key.NewBinding(
			key.WithKeys("shift+down", "ctrl+d"),
			key.WithHelp("Shift-↓/Ctrl-D", "Preview Down"),
		),
		EnterChild: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "Open Child"),
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
		Workspace: key.NewBinding(
			key.WithKeys("alt+w"),
			key.WithHelp("Alt-W", "Workspace"),
		),

		// Input
		Submit: key.NewBinding(
			key.WithKeys("alt+enter", "ctrl+s"),
			key.WithHelp("Alt-Enter", "Submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "Cancel"),
		),
	}
}

// ApplyShortcuts replaces action key bindings from a map of action name to key
// string. Intended for default mode only — vim mode key bindings are opinionated
// and not user-configurable. Returns an error if a shortcut collides with a
// reserved key or if two actions map to the same key. Unknown action names are
// silently ignored.
func (k *KeyMap) ApplyShortcuts(shortcuts map[string]string) error {
	if len(shortcuts) == 0 {
		return nil
	}

	bindings := map[string]*key.Binding{
		"refresh":    &k.Refresh,
		"filter":     &k.Filter,
		"assign":     &k.Assign,
		"transition": &k.Transition,
		"open":       &k.Open,
		"edit":       &k.Edit,
		"comment":    &k.Comment,
		"branch":     &k.Branch,
		"extract":    &k.Extract,
		"new":        &k.New,
		"workspace":  &k.Workspace,
	}

	// Shortcuts must use a modifier prefix (alt, ctrl, super, hyper) to avoid
	// conflicting with search input, which consumes bare characters.
	for name, keyStr := range shortcuts {
		if _, ok := bindings[name]; !ok {
			continue
		}
		if !hasModifierPrefix(keyStr) {
			return fmt.Errorf("shortcut %q: key %q must include a modifier (alt, ctrl, super, or hyper)", name, keyStr)
		}
	}

	// Collect reserved keys from non-configurable bindings.
	reserved := map[string]string{}
	for desc, b := range map[string]*key.Binding{
		"Quit":                &k.Quit,
		"Help":                &k.Help,
		"Navigate up":         &k.Up,
		"Navigate down":       &k.Down,
		"Jump to first":       &k.Home,
		"Jump to last":        &k.End,
		"Page up":             &k.PageUp,
		"Page down":           &k.PageDn,
		"Scroll preview up":   &k.PreviewUp,
		"Scroll preview down": &k.PreviewDown,
		"Enter child":         &k.EnterChild,
		"Submit":              &k.Submit,
		"Cancel/Escape":       &k.Cancel,
	} {
		for _, ks := range b.Keys() {
			reserved[ks] = desc
		}
	}

	// Check for collisions before applying any changes.
	seen := map[string]string{} // key string → action name
	for name, keyStr := range shortcuts {
		if _, ok := bindings[name]; !ok {
			continue
		}
		if desc, ok := reserved[keyStr]; ok {
			return fmt.Errorf("shortcut %q: key %q is reserved by %s", name, keyStr, desc)
		}
		if other, ok := seen[keyStr]; ok {
			return fmt.Errorf("shortcut %q: key %q already used by %q", name, keyStr, other)
		}
		seen[keyStr] = name
	}

	// Also check against existing action bindings that aren't being overridden.
	for name, b := range bindings {
		if _, overridden := shortcuts[name]; overridden {
			continue
		}
		for _, ks := range b.Keys() {
			if other, ok := seen[ks]; ok {
				return fmt.Errorf("shortcut %q: key %q already used by default binding for %q", other, ks, name)
			}
		}
	}

	for name, keyStr := range shortcuts {
		if b, ok := bindings[name]; ok {
			help := b.Help()
			*b = key.NewBinding(
				key.WithKeys(keyStr),
				key.WithHelp(formatKeyDisplay(keyStr), help.Desc),
			)
		}
	}
	return nil
}

// formatKeyDisplay converts a Bubble Tea key string (e.g., "ctrl+b") into
// the default mode display format: Title Case with hyphen separators (e.g., "Ctrl-B").
func formatKeyDisplay(keyStr string) string {
	parts := strings.Split(keyStr, "+")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "-")
}

// hasModifierPrefix reports whether a key string includes a non-shift modifier.
func hasModifierPrefix(keyStr string) bool {
	for _, prefix := range []string{"alt+", "ctrl+", "super+", "hyper+"} {
		if strings.HasPrefix(keyStr, prefix) {
			return true
		}
	}
	return false
}
