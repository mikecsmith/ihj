package tui

// Action represents a user-initiated command in the TUI.
// It decouples "what to do" from "which key was pressed", allowing
// multiple key binding schemes (default alt-keys, vim single-chars)
// to share the same action execution logic.
type Action int

const (
	ActionNone Action = iota
	ActionRefresh
	ActionFilter
	ActionAssign
	ActionTransition
	ActionOpen
	ActionEdit
	ActionComment
	ActionBranch
	ActionExtract
	ActionNew
	ActionWorkspace
)
