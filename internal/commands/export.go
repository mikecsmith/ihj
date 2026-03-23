package commands

import (
	"encoding/json"
	"fmt"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/jira"
)

func Export(app *App, boardSlug, filterName string) error {
	board, err := app.Config.ResolveBoard(boardSlug)
	if err != nil {
		return err
	}

	jql, err := config.BuildJQL(board, filterName, app.Config.FormattedCustomFields)
	if err != nil {
		return err
	}

	issues, err := jira.FetchAllIssues(app.Client, jql, app.Config.FormattedCustomFields)
	if err != nil {
		return err
	}

	hierarchy, hashes := jira.BuildExportHierarchy(issues)

	if err := jira.SaveExportState(app.CacheDir, board.Slug, hashes); err != nil {
		_, _ = fmt.Fprintf(app.Err, "Warning: could not save state file: %v\n", err)
	}

	meta := jira.BuildExportMetadata(board.Slug, board)
	output := map[string]any{"metadata": meta, "issues": hierarchy}

	enc := json.NewEncoder(app.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
