package jira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

)

const cacheTTL = 15 * time.Minute

type CachedData struct {
	Issues    []Issue
	FetchedAt time.Time
}

func LoadCache(cacheDir, slug, filter string) (*CachedData, error) {
	path := cachePath(cacheDir, slug, filter)

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cache miss: %w", err)
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil, fmt.Errorf("cache expired (%s old)", time.Since(info.ModTime()).Round(time.Second))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("corrupt cache: %w", err)
	}

	return &CachedData{Issues: issues, FetchedAt: info.ModTime()}, nil
}

func SaveCache(cacheDir, slug, filter string, issues []Issue) error {
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
