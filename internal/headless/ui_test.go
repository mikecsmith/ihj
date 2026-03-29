package headless_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/headless"
)

func TestReviewDiff_EmptyOptions(t *testing.T) {
	h := headless.NewHeadlessUI()

	got, err := h.ReviewDiff("Review", nil, []string{})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if want := -1; got != want {
		t.Errorf("chosen = %d, want %d", got, want)
	}
}
