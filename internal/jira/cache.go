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

// LoadCacheAnyAge loads cached data regardless of age. Returns nil on miss.
// Used for stale-while-revalidate: show stale data immediately while fetching fresh.
func LoadCacheAnyAge(cacheDir, slug, filter string) *CachedData {
	path := cachePath(cacheDir, slug, filter)

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var issues []Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil
	}

	return &CachedData{Issues: issues, FetchedAt: info.ModTime()}
}

// IsCacheFresh returns true if the cache file exists and is within TTL.
func IsCacheFresh(cacheDir, slug, filter string) bool {
	info, err := os.Stat(cachePath(cacheDir, slug, filter))
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) <= cacheTTL
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

func SaveExportState(cacheDir, slug string, hashes map[string]string) error {
	path := filepath.Join(cacheDir, fmt.Sprintf(".%s.state.json", slug))
	data, err := json.MarshalIndent(hashes, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func CacheAge(cacheDir, slug, filter string) int {
	info, err := os.Stat(cachePath(cacheDir, slug, filter))
	if err != nil {
		return -1
	}
	return int(time.Since(info.ModTime()).Seconds())
}

func cachePath(dir, slug, filter string) string {
	return filepath.Join(dir, fmt.Sprintf("%s_%s.json", slug, filter))
}
