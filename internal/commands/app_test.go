package commands

import (
	"os"
	"testing"
)

func TestCancelledError(t *testing.T) {
	err := &CancelledError{Operation: "test"}
	if err.Error() != "test cancelled" {
		t.Errorf("CancelledError.Error() = %q; want \"test cancelled\"", err.Error())
	}
	if !IsCancelled(err) {
		t.Errorf("IsCancelled(&CancelledError{}) = false; want true")
	}
	if IsCancelled(nil) {
		t.Errorf("IsCancelled(nil) = true; want false")
	}
	if IsCancelled(os.ErrNotExist) {
		t.Errorf("IsCancelled(os.ErrNotExist) = true; want false")
	}
}
