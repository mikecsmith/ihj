package commands

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
)

func Export(s *Session, workspaceSlug, filterName string) error {
	ws, err := s.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return err
	}

	// Export always fetches fresh data.
	items, err := s.Provider.Search(nil, filterName, true)
	if err != nil {
		return err
	}

	// Build tree from flat items.
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	roots := core.Roots(registry)

	// Build content hashes for apply safety.
	hashes := make(map[string]string, len(registry))
	for id, item := range registry {
		hashes[id] = item.ContentHash()
	}

	if err := saveState(s.CacheDir, ws.Slug, hashes); err != nil {
		_, _ = fmt.Fprintf(s.Err, "Warning: could not save state file: %v\n", err)
	}

	schema := core.ManifestSchema(ws)
	schemaPath, err := writeSchema(s.CacheDir, ws.Slug, core.ManifestStr, schema)
	if err != nil {
		_, _ = fmt.Fprintf(s.Err, "Warning: could not save manifest schema: %v\n", err)
	}

	meta := core.Metadata{
		Backend:    ws.Provider,
		Target:     ws.Slug,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}

	manifest := core.Manifest{
		Metadata: meta,
		Items:    roots,
	}

	if schemaPath != "" {
		absPath, _ := filepath.Abs(schemaPath)
		uriPath := filepath.ToSlash(absPath)
		fmt.Fprintf(s.Out, "# yaml-language-server: $schema=file://%s\n", uriPath)
	}

	enc := yaml.NewEncoder(s.Out, yaml.UseLiteralStyleIfMultiline(true))
	return enc.Encode(manifest)
}
