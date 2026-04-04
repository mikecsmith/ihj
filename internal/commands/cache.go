package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// writeSchema writes a JSON schema to the cache directory.
func writeSchema(cacheDir, provider, workspaceSlug, name string, schema any) (string, error) {
	filename := fmt.Sprintf("%s.%s.%s.schema.json", provider, name, workspaceSlug)
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

// saveState persists a string→string map (typically content hashes) to the cache directory.
func saveState(cacheDir, provider, slug string, state map[string]string) error {
	path := filepath.Join(cacheDir, fmt.Sprintf("%s.state.%s.json", provider, slug))
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}
