package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
)

// keys returns the map keys for error messages.
func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestExport_WritesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(client.SearchResponse{
			Issues: []client.Issue{
				{Key: "ENG-1", Fields: client.IssueFields{
					Summary:   "Test",
					IssueType: client.IssueType{ID: "10", Name: "Story"},
					Status:    client.Status{Name: "Open"},
					Priority:  client.Priority{Name: "Medium"},
					Created:   "2024-01-01T00:00:00.000+0000",
					Updated:   "2024-01-01T00:00:00.000+0000",
				}},
			},
			IsLast: true,
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	ui := &MockUI{}
	app := NewTestApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))
	app.CacheDir = t.TempDir()
	app.Out = &buf

	err := Export(app, "eng", "active")
	if err != nil {
		t.Fatal(err)
	}

	var output map[string]any
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := output["metadata"]; !ok {
		t.Errorf("Export output missing \"metadata\" key; got keys: %v", keys(output))
	}
	if _, ok := output["issues"]; !ok {
		t.Errorf("Export output missing \"issues\" key; got keys: %v", keys(output))
	}
}
