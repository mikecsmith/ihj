package commands

import (
	"fmt"
)

func Assign(app *App, issueKey string) error {
	if err := app.Provider.Assign(nil, issueKey); err != nil {
		app.UI.Notify("Error", fmt.Sprintf("Failed to assign %s.", issueKey))
		return err
	}

	app.UI.Notify("Assigned", fmt.Sprintf("Assigned %s to you.", issueKey))
	return nil
}
