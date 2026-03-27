package commands

import "fmt"

// RunDemo launches the TUI with synthetic data.
// The demo provider and workspace must already be configured on app
// (via demo.SetupConfig + newProvider in the composition root).
func RunDemo(app *App) error {
	if app.LaunchTUI == nil {
		return fmt.Errorf("TUI not available (LaunchTUI not configured)")
	}

	ws, err := app.Config.ResolveWorkspace("")
	if err != nil {
		return fmt.Errorf("demo workspace not configured: %w", err)
	}

	items, err := app.Provider.Search(nil, "active", nil)
	if err != nil {
		return fmt.Errorf("loading demo data: %w", err)
	}

	return app.LaunchTUI(&LaunchTUIData{
		App:       app,
		Workspace: ws,
		Filter:    "active",
		Items:     items,
	})
}
