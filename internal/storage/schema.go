package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteSchema writes a JSON schema to the cache directory.
func WriteSchema(cacheDir, workspaceSlug, name string, schema any) (string, error) {
	filename := fmt.Sprintf("%s.%s.schema.json", name, workspaceSlug)
	path := filepath.Join(cacheDir, filename)

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling %s schema: %w", name, err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("writing %s schema: %w", name, err)
	}

	return path, nil
}
