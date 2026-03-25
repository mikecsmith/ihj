package commands

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/core"
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

	schema := core.ManifestSchema(board)
	schemaPath, err := core.WriteSchema(app.CacheDir, board.Slug, core.ManifestStr, schema)
	if err != nil {
		_, _ = fmt.Fprintf(app.Err, "Warning: could not save manifest schema: %v\n", err)
	}

	meta := jira.BuildExportMetadata(board.Slug, board)

	manifest := core.Manifest{
		Metadata: meta,
		Items:    hierarchy,
	}

	if schemaPath != "" {
		absPath, _ := filepath.Abs(schemaPath)
		uriPath := filepath.ToSlash(absPath)
		fmt.Fprintf(app.Out, "# yaml-language-server: $schema=file://%s\n", uriPath)
	}

	enc := yaml.NewEncoder(app.Out, yaml.UseLiteralStyleIfMultiline(true))
	return enc.Encode(manifest)
}
