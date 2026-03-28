package commands_test

import (
	"os"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
)

func TestCancelledError(t *testing.T) {
	err := &commands.CancelledError{Operation: "test"}
	if err.Error() != "test cancelled" {
		t.Errorf("CancelledError.Error() = %q; want \"test cancelled\"", err.Error())
	}
	if !commands.IsCancelled(err) {
		t.Errorf("IsCancelled(&CancelledError{}) = false; want true")
	}
	if commands.IsCancelled(nil) {
		t.Errorf("IsCancelled(nil) = true; want false")
	}
	if commands.IsCancelled(os.ErrNotExist) {
		t.Errorf("IsCancelled(os.ErrNotExist) = true; want false")
	}
}
