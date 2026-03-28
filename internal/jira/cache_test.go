package jira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadCache(t *testing.T) {
	dir := t.TempDir()
	issues := []Issue{
		testIssue("C-1", "Cached", "Task", "1", "Open", "Medium", ""),
	}

	if err := SaveCache(dir, "board", "active", issues); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	loaded, err := LoadCache(dir, "board", "active")
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if len(loaded.Issues) != 1 || loaded.Issues[0].Key != "C-1" {
		t.Errorf("LoadCache().Issues = %v; want 1 issue with Key=\"C-1\"", loaded.Issues)
	}
}

func TestLoadCache_Missing(t *testing.T) {
	_, err := LoadCache(t.TempDir(), "none", "none")
	if err == nil {
		t.Error("LoadCache() error = nil; want non-nil for missing cache")
	}
}

func TestLoadCache_Corrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad_active.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	_, err := LoadCache(dir, "bad", "active")
	if err == nil {
		t.Error("LoadCache() error = nil; want non-nil for corrupt cache")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("LoadCache() error = %v; want substring \"corrupt\"", err)
	}
}

