package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

// Export writes the workspace's issue hierarchy as a YAML manifest to stdout.
// When full is true, extended and read-only fields are included.
func Export(ws *WorkspaceSession, filterName string, full bool) error {
	// Export always fetches fresh data.
	items, err := ws.Provider.Search(context.TODO(), filterName, true)
	if err != nil {
		return err
	}

	defs := ws.Provider.FieldDefinitions()

	// Build tree from flat items.
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	roots := core.Roots(registry)

	// Build content hashes for apply safety.
	hashes := make(map[string]string, len(registry))
	for id, item := range registry {
		hashes[id] = item.ContentHash()
	}

	if err := saveState(ws.Runtime.CacheDir, ws.Workspace.Slug, hashes); err != nil {
		_, _ = fmt.Fprintf(ws.Runtime.Err, "Warning: could not save state file: %v\n", err)
	}

	schema := core.ManifestSchema(ws.Workspace, defs)
	schemaPath, err := writeSchema(ws.Runtime.CacheDir, ws.Workspace.Slug, core.ManifestStr, schema)
	if err != nil {
		_, _ = fmt.Fprintf(ws.Runtime.Err, "Warning: could not save manifest schema: %v\n", err)
	}

	manifest := core.Manifest{
		Metadata: core.Metadata{
			Workspace:  ws.Workspace.Slug,
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Items: roots,
	}

	if schemaPath != "" {
		absPath, _ := filepath.Abs(schemaPath)
		uriPath := filepath.ToSlash(absPath)
		fmt.Fprintf(ws.Runtime.Out, "# yaml-language-server: $schema=file://%s\n", uriPath)
	}

	return core.EncodeManifest(ws.Runtime.Out, &manifest, defs, full, "yaml")
}
