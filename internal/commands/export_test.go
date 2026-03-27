package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/core"
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
	provider := &core.MockProvider{
		SearchReturn: []*core.WorkItem{
			{
				ID:      "ENG-1",
				Type:    "Story",
				Summary: "Test",
				Status:  "Open",
			},
		},
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	ui := &MockUI{}

	s := NewTestSession(ui)
	s.Provider = provider
	s.CacheDir = t.TempDir()
	s.Out = &outBuf
	s.Err = &errBuf

	err := Export(s, "eng", "active")
	if err != nil {
		t.Fatalf("Export() err = %v, want nil", err)
	}

	outputStr := outBuf.String()
	errStr := errBuf.String()

	if got, want := strings.HasPrefix(outputStr, "# yaml-language-server: $schema=file://"), true; got != want {
		t.Errorf("strings.HasPrefix(outputStr, \"# yaml-language-server...\") = %v, want %v\nStderr: %s\nOutput:\n%s", got, want, errStr, outputStr)
	}

	files, err := os.ReadDir(s.CacheDir)
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
