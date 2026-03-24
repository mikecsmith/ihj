package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newTestPopup() PopupModel {
	theme := DefaultTheme()
	styles := NewStyles(theme, nil)
	keys := DefaultKeyMap()
	p := NewPopupModel(styles, keys)
	p.SetSize(80, 30)
	return p
}

func downKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyDown}
}

func upKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyUp}
}

func escKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEscape}
}

func numKey(n rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: n}
}

func TestPopupSelect(t *testing.T) {
	t.Run("initially inactive", func(t *testing.T) {
		p := newTestPopup()
		if p.Active() {
			t.Error("Active() = true; want false")
		}
	})

	t.Run("show activates", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick one", []string{"A", "B", "C"})
		if !p.Active() {
			t.Error("Active() = false; want true")
		}
		if p.cursor != 0 {
			t.Errorf("cursor = %d; want 0", p.cursor)
		}
	})

	t.Run("down moves cursor", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B", "C"})
		p.Update(downKey())
		if p.cursor != 1 {
			t.Errorf("cursor = %d; want 1", p.cursor)
		}
	})

	t.Run("up at top stays", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B", "C"})
		p.Update(upKey())
		if p.cursor != 0 {
			t.Errorf("cursor = %d; want 0", p.cursor)
		}
	})

	t.Run("down at bottom clamps", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B"})
		p.Update(downKey())
		p.Update(downKey())
		if p.cursor != 1 {
			t.Errorf("cursor = %d; want 1", p.cursor)
		}
	})

	t.Run("enter confirms", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B"})
		_, result := p.Update(enterKey())
		if result == nil {
			t.Fatal("Update(enter) result = nil; want non-nil")
		}
		if result.Index != 0 {
			t.Errorf("result.Index = %d; want 0", result.Index)
		}
		if result.Value != "A" {
			t.Errorf("result.Value = %q; want %q", result.Value, "A")
		}
		if result.Canceled {
			t.Error("result.Canceled = true; want false")
		}
	})

	t.Run("enter after down", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B"})
		p.Update(downKey())
		_, result := p.Update(enterKey())
		if result == nil {
			t.Fatal("Update(enter) result = nil; want non-nil")
		}
		if result.Index != 1 {
			t.Errorf("result.Index = %d; want 1", result.Index)
		}
		if result.Value != "B" {
			t.Errorf("result.Value = %q; want %q", result.Value, "B")
		}
	})

	t.Run("escape cancels", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B"})
		_, result := p.Update(escKey())
		if result == nil {
			t.Fatal("Update(esc) result = nil; want non-nil")
		}
		if !result.Canceled {
			t.Error("result.Canceled = false; want true")
		}
		if result.Index != -1 {
			t.Errorf("result.Index = %d; want -1", result.Index)
		}
	})

	t.Run("number key shortcut", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B", "C"})
		_, result := p.Update(numKey('2'))
		if result == nil {
			t.Fatal("Update('2') result = nil; want non-nil")
		}
		if result.Index != 1 {
			t.Errorf("result.Index = %d; want 1", result.Index)
		}
		if result.Value != "B" {
			t.Errorf("result.Value = %q; want %q", result.Value, "B")
		}
	})

	t.Run("number out of range", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B"})
		_, result := p.Update(numKey('5'))
		if result != nil {
			t.Errorf("Update('5') result = %+v; want nil", result)
		}
	})

	t.Run("close deactivates", func(t *testing.T) {
		p := newTestPopup()
		p.ShowSelect("test", "Pick", []string{"A"})
		p.Close()
		if p.Active() {
			t.Error("Active() = true after Close(); want false")
		}
	})
}

func TestPopupInput(t *testing.T) {
	t.Run("show activates", func(t *testing.T) {
		p := newTestPopup()
		p.ShowInput("comment", "Comment", "Type here...")
		if !p.Active() {
			t.Error("Active() = false; want true")
		}
	})

	t.Run("escape cancels", func(t *testing.T) {
		p := newTestPopup()
		p.ShowInput("comment", "Comment", "Type here...")
		_, result := p.Update(escKey())
		if result == nil {
			t.Fatal("Update(esc) result = nil; want non-nil")
		}
		if !result.Canceled {
			t.Error("result.Canceled = false; want true")
		}
	})

	t.Run("submit empty cancels", func(t *testing.T) {
		p := newTestPopup()
		p.ShowInput("comment", "Comment", "Type here...")
		// Submit without typing — ctrl+s matches the Submit binding
		_, result := p.Update(ctrlKey('s'))
		if result == nil {
			t.Fatal("Update(ctrl+s) result = nil; want non-nil")
		}
		if !result.Canceled {
			t.Error("result.Canceled = false; want true (empty input)")
		}
	})
}
