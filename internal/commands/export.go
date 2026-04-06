package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/encoding"
)

// Export writes the workspace's issue hierarchy as a YAML manifest to stdout.
// When full is true, extended and read-only fields are included.
func Export(ctx context.Context, ws *WorkspaceSession, filterName string, full bool) error {
	// Export always fetches fresh data.
	items, err := ws.Provider.Search(ctx, filterName, true)
	if err != nil {
		return err
	}

	defs := ws.Workspace.AllFieldDefs()

	// Build tree from flat items.
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	roots := core.Roots(registry)

	// Build content hashes for apply safety.
	hashes := make(map[string]string, len(registry))
	for id, item := range registry {
		hashes[id] = item.ContentHash()
	}

	if err := saveState(ws.Runtime.CacheDir, ws.Workspace.Provider, ws.Workspace.Slug, hashes); err != nil {
		_, _ = fmt.Fprintf(ws.Runtime.Err, "Warning: could not save state file: %v\n", err)
	}

	schema := encoding.ManifestSchema(ws.Workspace, defs)
	schemaPath, err := writeSchema(ws.Runtime.CacheDir, ws.Workspace.Provider, ws.Workspace.Slug, encoding.ManifestStr, schema)
	if err != nil {
		_, _ = fmt.Fprintf(ws.Runtime.Err, "Warning: could not save manifest schema: %v\n", err)
	}

	manifest := encoding.Manifest{
		Metadata: encoding.Metadata{
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

	return encoding.EncodeManifest(ws.Runtime.Out, &manifest, defs, full, "yaml")
}
