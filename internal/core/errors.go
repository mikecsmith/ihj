package core

import "errors"

// CancelledError indicates the user intentionally cancelled an operation.
// The CLI should exit cleanly (code 0) rather than printing an error.
type CancelledError struct {
	Operation string
}

func (e *CancelledError) Error() string {
	return e.Operation + " cancelled"
}

// IsCancelled checks whether an error is a user cancellation.
func IsCancelled(err error) bool {
	var ce *CancelledError
	return errors.As(err, &ce)
}
