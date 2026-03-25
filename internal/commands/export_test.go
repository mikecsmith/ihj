package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
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

func TestExport_WritesYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(client.SearchResponse{
			Issues: []client.Issue{
				{Key: "ENG-1", Fields: client.IssueFields{
					Summary:   "Test",
					IssueType: client.IssueType{ID: "10", Name: "Story"},
					Status:    client.Status{Name: "Open"},
				}},
			},
			IsLast: true,
		})
	}))
	defer srv.Close()

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	ui := &MockUI{}

	app := NewTestApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))
	app.CacheDir = t.TempDir()
	app.Out = &outBuf
	app.Err = &errBuf

	err := Export(app, "eng", "active")
	if err != nil {
		t.Fatalf("Export() err = %v, want nil", err)
	}

	outputStr := outBuf.String()
	errStr := errBuf.String()

	if got, want := strings.HasPrefix(outputStr, "# yaml-language-server: $schema=file://"), true; got != want {
		t.Errorf("strings.HasPrefix(outputStr, \"# yaml-language-server...\") = %v, want %v\nStderr: %s\nOutput:\n%s", got, want, errStr, outputStr)
	}

	files, err := os.ReadDir(app.CacheDir)
	if err != nil {
		t.Fatalf("os.ReadDir() err = %v, want nil", err)
	}

	schemaFound := false
	var foundNames []string
	for _, f := range files {
		foundNames = append(foundNames, f.Name())
		if strings.HasSuffix(f.Name(), ".schema.json") || strings.HasSuffix(f.Name(), ".json") {
			schemaFound = true
			break
		}
	}
	if got, want := schemaFound, true; got != want {
		t.Errorf("schema file found = %v, want %v\nFiles in dir: %v\nStderr: %s", got, want, foundNames, errStr)
	}

	var output map[string]any
	if err := yaml.Unmarshal(outBuf.Bytes(), &output); err != nil {
		t.Fatalf("yaml.Unmarshal() err = %v, want nil\nOutput:\n%s", err, outputStr)
	}

	if _, ok := output["metadata"]; !ok {
		t.Errorf("output has key %q = false, want true\nKeys: %v", "metadata", keys(output))
	}

	if _, ok := output["items"]; !ok {
		t.Errorf("output has key %q = false, want true\nKeys: %v", "items", keys(output))
	}
}
