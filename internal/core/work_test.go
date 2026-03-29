package core

import (
	"encoding/json"
	"testing"
)

func TestManifestSchema_Validation(t *testing.T) {
	ws := &Workspace{
		Types:    []TypeConfig{{Name: "Epic"}, {Name: "Story"}, {Name: "Task"}},
		Statuses: []string{"Backlog", "Done"},
	}

	sch := ManifestSchema(ws, nil)

	resolved, err := sch.Resolve(nil)
	if err != nil {
		t.Fatalf("Failed to resolve schema: %v", err)
	}

	// TEST: Valid Nested JSON payload (using the new Manifest structure)
	validJSON := `{
		"metadata": {
			"workspace": "eng"
		},
		"items": [
			{
				"type": "Epic",
				"summary": "Main Epic",
				"status": "Backlog",
				"children": [
					{
						"type": "Story",
						"summary": "Child Story",
						"status": "Backlog"
					}
				]
			}
		]
	}`

	var inst any
	if err := json.Unmarshal([]byte(validJSON), &inst); err != nil {
		t.Fatalf("Failed to unmarshal valid JSON setup: %v", err)
	}

	if err := resolved.Validate(inst); err != nil {
		t.Errorf("Expected valid JSON to pass, got error: %v", err)
	}
}

func TestWorkItem_Hashing(t *testing.T) {
	item1 := &WorkItem{
		ID:      "ENG-1",
		Type:    "Story",
		Summary: "Hash Test",
		Status:  "To Do",
		Fields:  map[string]any{"priority": "High", "sprint": 1},
	}

	// 1. Test Determinism
	hashA := item1.ContentHash()
	hashB := item1.ContentHash()
	if hashA != hashB {
		t.Errorf("ContentHash is not deterministic: %s != %s", hashA, hashB)
	}

	// 2. Test Core Field Change Detection
	item1.Summary = "Updated Hash Test"
	hashC := item1.ContentHash()
	if hashA == hashC {
		t.Error("ContentHash did not change when Summary was updated")
	}

	// 3. Test Flex Bucket Change Detection
	item1.Summary = "Hash Test" // revert
	item1.Fields["priority"] = "Low"
	hashD := item1.ContentHash()
	if hashA == hashD {
		t.Error("ContentHash did not change when Fields map was updated")
	}

	// 4. Test StateHash (Idempotency)
	state1 := item1.StateHash("PARENT-A")
	state2 := item1.StateHash("PARENT-B")
	if state1 == state2 {
		t.Error("StateHash did not change when parentID was different")
	}

	// Ensure ID does NOT affect StateHash (since ID doesn't exist during creation)
	item1.ID = "NEW-ID-2"
	state3 := item1.StateHash("PARENT-A")
	if state1 != state3 {
		t.Error("StateHash should remain identical even if ID changes")
	}
}
