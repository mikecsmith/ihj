// Package commands implements the core business logic for ihj.
//
// It orchestrates interactions between the backend provider, the local
// configuration, and the user interface (both headless and interactive
// TUI modes). The Cobra command tree lives in cmd/ihj/.
package commands

import (
	"errors"
	"io"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/storage"
)

// Session holds all dependencies for command execution.
// It is created once at startup and passed to all commands.
type Session struct {
	Config   *storage.AppConfig
	Provider core.Provider
	UI       UI
	CacheDir string
	Out      io.Writer
	Err      io.Writer

	// LaunchTUI is set by main.go to the Bubble Tea launcher.
	// This avoids the commands package importing bubbletea directly.
	LaunchTUI func(data *LaunchTUIData) error
}

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

