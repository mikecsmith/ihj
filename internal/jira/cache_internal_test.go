package jira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateMetaCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	meta := &cachedCreateMeta{
		ServerAlias: "my-jira",
		ProjectKey:  "PROJ",
		Types: map[string][]createMetaField{
			"10001": {
				{
					FieldID:  "priority",
					Key:      "priority",
					Name:     "Priority",
					Required: false,
					Schema:   fieldSchema{Type: "priority", System: "priority"},
					AllowedValues: json.RawMessage(`[
						{"id":"1","name":"Highest"},
						{"id":"2","name":"High"}
					]`),
				},
				{
					FieldID:  "customfield_10016",
					Key:      "customfield_10016",
					Name:     "Story Points",
					Required: true,
					Schema:   fieldSchema{Type: "number", CustomID: 10016},
				},
			},
		},
	}

	if err := saveCreateMetaCache(dir, "my-jira", "PROJ", meta); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify file exists at expected path.
	path := filepath.Join(dir, ".meta_my-jira_PROJ.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache file not found: %v", err)
	}

	got, err := loadCreateMetaCache(dir, "my-jira", "PROJ", DefaultMetaCacheTTL)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got.ServerAlias != "my-jira" {
		t.Errorf("ServerAlias = %q", got.ServerAlias)
	}
	if got.ProjectKey != "PROJ" {
		t.Errorf("ProjectKey = %q", got.ProjectKey)
	}

	fields, ok := got.Types["10001"]
	if !ok {
		t.Fatal("missing type 10001")
	}
	if len(fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(fields))
	}
	if fields[0].Key != "priority" {
		t.Errorf("fields[0].Key = %q", fields[0].Key)
	}
	if !fields[1].Required {
		t.Error("fields[1].Required should be true")
	}
}

func TestCreateMetaCache_Expiry(t *testing.T) {
	dir := t.TempDir()

	meta := &cachedCreateMeta{
		ServerAlias: "srv",
		ProjectKey:  "P",
		Types:       map[string][]createMetaField{},
	}

	if err := saveCreateMetaCache(dir, "srv", "P", meta); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Backdate the file to trigger expiry.
	path := createMetaCachePath(dir, "srv", "P")
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	_, err := loadCreateMetaCache(dir, "srv", "P", DefaultMetaCacheTTL)
	if err == nil {
		t.Fatal("expected expiry error")
	}
}

func TestCreateMetaCache_Miss(t *testing.T) {
	dir := t.TempDir()

	_, err := loadCreateMetaCache(dir, "nonexistent", "NOPE", DefaultMetaCacheTTL)
	if err == nil {
		t.Fatal("expected miss error")
	}
}
