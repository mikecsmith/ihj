package jira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type cachedData struct {
	Issues    []issue
	FetchedAt time.Time
}

func loadCache(cacheDir, slug, filter string, ttl time.Duration) (*cachedData, error) {
	path := cachePath(cacheDir, slug, filter)

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cache miss: %w", err)
	}
	if time.Since(info.ModTime()) > ttl {
		return nil, fmt.Errorf("cache expired (%s old)", time.Since(info.ModTime()).Round(time.Second))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	var issues []issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("corrupt cache: %w", err)
	}

	return &cachedData{Issues: issues, FetchedAt: info.ModTime()}, nil
}

func saveCache(cacheDir, slug, filter string, issues []issue) error {
	data, err := json.Marshal(issues)
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}
	if err := os.WriteFile(cachePath(cacheDir, slug, filter), data, 0o644); err != nil {
		return fmt.Errorf("writing cache: %w", err)
	}
	return nil
}

func cachePath(dir, slug, filter string) string {
	return filepath.Join(dir, fmt.Sprintf("%s_%s.json", slug, filter))
}

// DefaultMetaCacheTTL is the default time-to-live for createmeta cache files.
// Field metadata changes infrequently (admin-level config), so 24 hours is
// a sensible default.
const DefaultMetaCacheTTL = 24 * time.Hour

// cachedCreateMeta holds the per-type field metadata from the createmeta API.
// Scoped by ServerAlias + ProjectKey so workspaces sharing the same project
// share the cache.
type cachedCreateMeta struct {
	ServerAlias string                       `json:"serverAlias"`
	ProjectKey  string                       `json:"projectKey"`
	Types       map[string][]createMetaField `json:"types"` // issueTypeID → fields
}

func loadCreateMetaCache(cacheDir, serverAlias, projectKey string, ttl time.Duration) (*cachedCreateMeta, error) {
	path := createMetaCachePath(cacheDir, serverAlias, projectKey)

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("meta cache miss: %w", err)
	}
	if time.Since(info.ModTime()) > ttl {
		return nil, fmt.Errorf("meta cache expired (%s old)", time.Since(info.ModTime()).Round(time.Second))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading meta cache: %w", err)
	}

	var meta cachedCreateMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("corrupt meta cache: %w", err)
	}

	return &meta, nil
}

func saveCreateMetaCache(cacheDir, serverAlias, projectKey string, meta *cachedCreateMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling meta cache: %w", err)
	}
	if err := os.WriteFile(createMetaCachePath(cacheDir, serverAlias, projectKey), data, 0o644); err != nil {
		return fmt.Errorf("writing meta cache: %w", err)
	}
	return nil
}

func createMetaCachePath(dir, serverAlias, projectKey string) string {
	return filepath.Join(dir, fmt.Sprintf(".meta_%s_%s.json", serverAlias, projectKey))
}
