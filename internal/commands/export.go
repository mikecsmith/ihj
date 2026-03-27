package commands

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/jira"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/storage"
)

func Export(app *App, workspaceSlug, filterName string) error {
	ws, err := app.Config.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return err
	}

	jiraCfg := ws.ProviderConfig.(*jira.Config)

	jql, err := jira.BuildJQL(ws, jiraCfg, filterName)
	if err != nil {
		return err
	}

	issues, err := jira.FetchAllIssues(app.Client, jql, jiraCfg.FormattedCustomFields)
	if err != nil {
		return err
	}

	hierarchy, hashes := jira.BuildExportHierarchy(issues)

	if err := jira.SaveExportState(app.CacheDir, ws.Slug, hashes); err != nil {
		_, _ = fmt.Fprintf(app.Err, "Warning: could not save state file: %v\n", err)
	}

	schema := core.ManifestSchema(ws)
	schemaPath, err := storage.WriteSchema(app.CacheDir, ws.Slug, core.ManifestStr, schema)
	if err != nil {
		_, _ = fmt.Fprintf(app.Err, "Warning: could not save manifest schema: %v\n", err)
	}

	meta := jira.BuildExportMetadata(ws.Slug, jiraCfg)

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
