package tui_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/tui"
)

func TestReviewDiff_EmptyOptions(t *testing.T) {
	b := tui.NewBubbleTeaUI()

	got, err := b.ReviewDiff("Review", nil, []string{})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if want := -1; got != want {
		t.Errorf("chosen = %d, want %d", got, want)
	}
}
