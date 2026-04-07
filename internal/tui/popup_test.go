package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mikecsmith/ihj/internal/terminal"
	"github.com/mikecsmith/ihj/internal/tui"
)

func bbDownKey() tea.KeyPressMsg  { return tea.KeyPressMsg{Code: tea.KeyDown} }
func bbUpKey() tea.KeyPressMsg    { return tea.KeyPressMsg{Code: tea.KeyUp} }
func bbEscKey() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyEscape} }
func bbEnterKey() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
func bbNumKey(n rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: n}
}
func bbCtrlKey(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch, Mod: tea.ModCtrl}
}

func newBlackboxTestPopup() tui.PopupModel {
	theme := terminal.DefaultTheme()
	styles := terminal.NewStyles(theme, nil, "")
	keys := terminal.DefaultKeyMap()
	p := tui.NewPopupModel(styles, keys)
	p.SetSize(80, 30)
	return p
}

func TestPopupSelectBlackbox(t *testing.T) {
	t.Run("initially inactive", func(t *testing.T) {
		p := newBlackboxTestPopup()
		if p.Active() {
			t.Error("Active() = true; want false")
		}
	})

	t.Run("ShowSelect activates", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowSelect("test", "Pick one", []string{"A", "B", "C"})
		if !p.Active() {
			t.Error("Active() = false; want true")
		}
	})

	t.Run("down+enter selects correct item", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B", "C"})
		p.Update(bbDownKey())
		_, result := p.Update(bbEnterKey())
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

	t.Run("up at top stays at first item", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B", "C"})
		p.Update(bbUpKey())
		_, result := p.Update(bbEnterKey())
		if result == nil {
			t.Fatal("Update(enter) result = nil; want non-nil")
		}
		if result.Index != 0 {
			t.Errorf("result.Index = %d; want 0", result.Index)
		}
	})

	t.Run("enter confirms with correct PopupResult", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowSelect("myaction", "Pick", []string{"X", "Y"})
		_, result := p.Update(bbEnterKey())
		if result == nil {
			t.Fatal("Update(enter) result = nil; want non-nil")
		}
		if result.ID != "myaction" {
			t.Errorf("result.ID = %q; want %q", result.ID, "myaction")
		}
		if result.Index != 0 {
			t.Errorf("result.Index = %d; want 0", result.Index)
		}
		if result.Value != "X" {
			t.Errorf("result.Value = %q; want %q", result.Value, "X")
		}
		if result.Canceled {
			t.Error("result.Canceled = true; want false")
		}
	})

	t.Run("escape cancels", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B"})
		_, result := p.Update(bbEscKey())
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
		p := newBlackboxTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B", "C"})
		_, result := p.Update(bbNumKey('2'))
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
		p := newBlackboxTestPopup()
		p.ShowSelect("test", "Pick", []string{"A", "B"})
		_, result := p.Update(bbNumKey('5'))
		if result != nil {
			t.Errorf("Update('5') result = %+v; want nil", result)
		}
	})

	t.Run("Close deactivates", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowSelect("test", "Pick", []string{"A"})
		p.Close()
		if p.Active() {
			t.Error("Active() = true after Close(); want false")
		}
	})
}

func TestPopupInputBlackbox(t *testing.T) {
	t.Run("ShowInput activates", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowInput("comment", "Comment", "Type here...")
		if !p.Active() {
			t.Error("Active() = false; want true")
		}
	})

	t.Run("input escape cancels", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowInput("comment", "Comment", "Type here...")
		_, result := p.Update(bbEscKey())
		if result == nil {
			t.Fatal("Update(esc) result = nil; want non-nil")
		}
		if !result.Canceled {
			t.Error("result.Canceled = false; want true")
		}
	})

	t.Run("input submit empty cancels", func(t *testing.T) {
		p := newBlackboxTestPopup()
		p.ShowInput("comment", "Comment", "Type here...")
		// Submit without typing -- ctrl+s matches the Submit binding
		_, result := p.Update(bbCtrlKey('s'))
		if result == nil {
			t.Fatal("Update(ctrl+s) result = nil; want non-nil")
		}
		if !result.Canceled {
			t.Error("result.Canceled = false; want true (empty input)")
		}
	})
}

// ── Hint-key shortcut tests ─────────────────────────────────────

func TestPopupSelect_HintKeyQuickSelect(t *testing.T) {
	makeItems := func(count int) []string {
		items := make([]string, count)
		for i := range items {
			items[i] = fmt.Sprintf("Item%d", i+1)
		}
		return items
	}

	tests := []struct {
		name      string
		count     int    // number of options in the popup
		key       rune   // hint key to press
		wantIndex int    // expected result index, -1 means ignored
		wantValue string // expected result value (empty when ignored)
	}{
		{"digit 1 selects first", 5, '1', 0, "Item1"},
		{"digit 5 selects fifth", 5, '5', 4, "Item5"},
		{"digit 9 selects ninth", 10, '9', 8, "Item9"},
		{"digit 0 selects tenth", 10, '0', 9, "Item10"},
		{"letter a selects eleventh", 13, 'a', 10, "Item11"},
		{"letter c selects thirteenth", 13, 'c', 12, "Item13"},
		{"digit beyond count ignored", 2, '3', -1, ""},
		{"letter beyond count ignored", 2, 'a', -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			popup := newBlackboxTestPopup()
			popup.ShowSelect("transition", "Pick", makeItems(tt.count))

			_, result := popup.Update(bbNumKey(tt.key))
			if tt.wantIndex == -1 {
				if result != nil {
					t.Errorf("pressing %q should be ignored, got index %d", string(tt.key), result.Index)
				}
				return
			}
			if result == nil {
				t.Fatalf("pressing %q returned nil; want index %d", string(tt.key), tt.wantIndex)
			}
			if result.Index != tt.wantIndex {
				t.Errorf("result.Index = %d; want %d", result.Index, tt.wantIndex)
			}
			if result.Value != tt.wantValue {
				t.Errorf("result.Value = %q; want %q", result.Value, tt.wantValue)
			}
		})
	}
}

func TestPopupSelect_HintKeysDisplayedInView(t *testing.T) {
	items := make([]string, 12)
	for i := range items {
		items[i] = fmt.Sprintf("Option%d", i+1)
	}
	popup := newBlackboxTestPopup()
	popup.ShowSelect("transition", "Pick", items)
	view := stripANSI(popup.View())

	// Hints follow keyboard order: 1-9, 0, a, b.
	for _, hint := range "1234567890ab" {
		if !strings.Contains(view, string(hint)) {
			t.Errorf("view missing hint key %q", string(hint))
		}
	}
}

// ── Content-assertion tests for popup rendering ──────────────────

func TestPopupSelect_ViewContainsTitleAndAllItems(t *testing.T) {
	p := newBlackboxTestPopup()
	items := []string{"To Do", "In Progress", "In Review", "Done"}
	p.ShowSelect("transition", "Transition: ENG-100", items)

	view := stripANSI(p.View())

	if !strings.Contains(view, "Transition: ENG-100") {
		t.Error("select popup should show the title")
	}
	for _, it := range items {
		if !strings.Contains(view, it) {
			t.Errorf("select popup view missing item %q", it)
		}
	}
}

func TestPopupSelect_LongListFitsInViewport(t *testing.T) {
	// With a list longer than the viewport can show, the popup must
	// still render the currently-selected item and have the visible-
	// window math come from CalculateWindow (already rule-tested).
	p := newBlackboxTestPopup()
	items := []string{
		"Backlog", "Refinement", "Ready for Dev", "To Do", "In Progress",
		"In Review", "QA", "Staging", "Ready for Release", "Done",
		"Closed", "Won't Fix",
	}
	p.ShowSelect("transition", "Long", items)

	view := stripANSI(p.View())
	// First item should be visible at start.
	if !strings.Contains(view, items[0]) {
		t.Errorf("long list should show first item %q initially", items[0])
	}
	if !strings.Contains(view, "Long") {
		t.Error("long list popup missing title")
	}
}

func TestPopupInput_ViewContainsTitleAndPlaceholder(t *testing.T) {
	p := newBlackboxTestPopup()
	p.ShowInput("comment", "Comment: ENG-100", "Type your comment...")

	view := stripANSI(p.View())
	if !strings.Contains(view, "Comment: ENG-100") {
		t.Error("input popup should show the title")
	}
	if !strings.Contains(view, "Type your comment") {
		t.Error("input popup should show the placeholder text when empty")
	}
}
