package commands

import "fmt"

// RunDemo launches the TUI with synthetic data.
// The demo provider and workspace must already be configured on the session
// (via demo.SetupConfig + newProvider in the composition root).
func RunDemo(s *Session) error {
	if s.LaunchTUI == nil {
		return fmt.Errorf("TUI not available (LaunchTUI not configured)")
	}

	ws, err := s.Config.ResolveWorkspace("")
	if err != nil {
		return fmt.Errorf("demo workspace not configured: %w", err)
	}

	items, err := s.Provider.Search(nil, "active", nil)
	if err != nil {
		return fmt.Errorf("loading demo data: %w", err)
	}

	return s.LaunchTUI(&LaunchTUIData{
		Session:   s,
		Workspace: ws,
		Filter:    "active",
		Items:     items,
	})
}
