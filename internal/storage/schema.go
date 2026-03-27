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

// SaveState persists a string→string map (typically content hashes) to the cache directory.
func SaveState(cacheDir, slug string, state map[string]string) error {
	path := filepath.Join(cacheDir, fmt.Sprintf(".%s.state.json", slug))
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadState reads a previously saved state map from the cache directory.
// Returns an empty map if the file doesn't exist.
func LoadState(cacheDir, slug string) map[string]string {
	state := make(map[string]string)
	path := filepath.Join(cacheDir, fmt.Sprintf(".%s.state.json", slug))
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	return state
}
