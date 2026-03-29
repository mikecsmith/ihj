package commands_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/testutil"
)

func TestExport_WritesYAML(t *testing.T) {
	provider := &testutil.MockProvider{
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
	ui := &testutil.MockUI{}

	ws := testutil.NewTestSession(ui)
	ws.Provider = provider
	ws.Runtime.CacheDir = t.TempDir()
	ws.Runtime.Out = &outBuf
	ws.Runtime.Err = &errBuf

	err := commands.Export(ws, "default", false)
	if err != nil {
		t.Fatalf("Export() err = %v, want nil", err)
	}

	outputStr := outBuf.String()
	errStr := errBuf.String()

	if got, want := strings.HasPrefix(outputStr, "# yaml-language-server: $schema=file://"), true; got != want {
		t.Errorf("strings.HasPrefix(outputStr, \"# yaml-language-server...\") = %v, want %v\nStderr: %s\nOutput:\n%s", got, want, errStr, outputStr)
	}

	files, err := os.ReadDir(ws.Runtime.CacheDir)
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

	mapKeys := func(m map[string]any) []string {
		ks := make([]string, 0, len(m))
		for k := range m {
			ks = append(ks, k)
		}
		return ks
	}

	if _, ok := output["metadata"]; !ok {
		t.Errorf("output has key %q = false, want true\nKeys: %v", "metadata", mapKeys(output))
	}

	if _, ok := output["items"]; !ok {
		t.Errorf("output has key %q = false, want true\nKeys: %v", "items", mapKeys(output))
	}
}
