// Package commands implements the core business logic for ihj.
//
// It orchestrates interactions between the backend provider, the local
// configuration, and the user interface (both headless and interactive
// TUI modes). The Cobra command tree lives in cmd/ihj/.
package commands

import (
	"fmt"
	"io"

	"github.com/mikecsmith/ihj/internal/core"
)

// Session holds all dependencies for command execution.
// It is created once at startup and passed to all commands.
type Session struct {
	Theme            string
	DefaultWorkspace string
	Workspaces       map[string]*core.Workspace
	Provider         core.Provider
	UI               UI
	CacheDir         string
	Out              io.Writer
	Err              io.Writer

	// Launcher starts the full-screen interactive UI. Set by main.go
	// to avoid the commands package importing the tui package directly.
	Launcher UILauncher
}

// ResolveWorkspace returns the workspace for the given slug, falling back
// to DefaultWorkspace. Returns an error if neither is found.
func (s *Session) ResolveWorkspace(slug string) (*core.Workspace, error) {
	if slug == "" {
		slug = s.DefaultWorkspace
	}
	if slug == "" {
		return nil, fmt.Errorf("no workspace specified and 'default_workspace' not set in config")
	}
	ws, ok := s.Workspaces[slug]
	if !ok {
		return nil, fmt.Errorf("workspace '%s' not found in config", slug)
	}
	return ws, nil
}

// ResolveFilter returns the effective filter name, falling back to "active".
func (s *Session) ResolveFilter(name string) string {
	if name != "" {
		return name
	}
	return "active"
}

// CancelledError is an alias for core.CancelledError for backward compatibility.
type CancelledError = core.CancelledError

// IsCancelled checks whether an error is a user cancellation.
func IsCancelled(err error) bool {
	return core.IsCancelled(err)
}

