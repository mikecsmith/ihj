package tui

import "testing"

func TestNewPromptModel_InitDoesNotPanic(t *testing.T) {
	m := newPromptModel("test prompt", DefaultKeyMap())
	// Init calls textinput.Focus which triggers cursor.Blink.
	// This must not panic with a nil pointer dereference.
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() returned nil, expected blink command")
	}
}
