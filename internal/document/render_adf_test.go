package document

import (
	"encoding/json"
	"testing"
)

func TestRenderADF_NilNode(t *testing.T) {
	out, err := RenderADF(nil)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["type"] != "doc" {
		t.Errorf("nil should produce empty doc, got %v", parsed)
	}
}
