package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/testutil"
)

// setupApplyTest scaffolds the test environment for Apply tests.
func setupApplyTest(t *testing.T, payload core.Manifest, seedItems []*core.WorkItem) (*commands.Runtime, commands.WorkspaceSessionFactory, *testutil.MockUI, string) {
	t.Helper()

	dir := t.TempDir()

	cacheDir := filepath.Join(dir, "cache")
	_ = os.MkdirAll(cacheDir, 0o755)

	mp := &testutil.MockProvider{
		Registry:     make(map[string]*core.WorkItem),
		CreatePrefix: "ENG",
	}
	for _, item := range seedItems {
		mp.Registry[item.ID] = item
	}

	// Write the manifest using EncodeManifest so field defs route correctly.
	inputFile := filepath.Join(dir, "import.yaml")
	f, err := os.Create(inputFile)
	if err != nil {
		t.Fatalf("creating input file: %v", err)
	}
	defs := mp.FieldDefinitions()
	if err := core.EncodeManifest(f, &payload, defs, true, "yaml"); err != nil {
		f.Close()
		t.Fatalf("encoding manifest: %v", err)
	}
	f.Close()

	ui := &testutil.MockUI{}
	rt := testutil.NewTestRuntime(ui)
	rt.CacheDir = cacheDir

	factory := func(slug string) (*commands.WorkspaceSession, error) {
		ws := testutil.TestWorkspace()
		return &commands.WorkspaceSession{
			Runtime:   rt,
			Workspace: ws,
			Provider:  mp,
		}, nil
	}

	return rt, factory, ui, inputFile
}

func TestApply(t *testing.T) {
	tests := []struct {
		name              string
		payload           core.Manifest
		seedItems         []*core.WorkItem
		userChoice        int // 0 = Apply/Create, 1 = Accept Remote, 2 = Skip, 3 = Abort
		wantErr           bool
		errMatch          string
		checkFileContains string
	}{
		{
			name: "Validation Failure - Invalid Type",
			payload: core.Manifest{
				Metadata: core.Metadata{Workspace: "eng"},
				Items: []*core.WorkItem{
					{Summary: "Invalid", Type: "MagicType", Status: "To Do"},
				},
			},
			wantErr:  true,
			errMatch: "validation failed",
		},
		{
			name: "Successful Creation Flow",
			payload: core.Manifest{
				Metadata: core.Metadata{Workspace: "eng"},
				Items: []*core.WorkItem{
					{Summary: "New Story", Type: "Story", Status: "To Do"},
				},
			},
			userChoice:        0,
			checkFileContains: "ENG-",
		},
		{
			name: "Idempotency - No Changes Found",
			seedItems: []*core.WorkItem{
				{ID: "ENG-1", Summary: "Same", Type: "Story", Status: "To Do"},
			},
			payload: core.Manifest{
				Metadata: core.Metadata{Workspace: "eng"},
				Items: []*core.WorkItem{
					{ID: "ENG-1", Summary: "Same", Type: "Story", Status: "To Do"},
				},
			},
			wantErr: false,
		},
		{
			name: "Successful Update Flow (Apply Changes)",
			seedItems: []*core.WorkItem{
				{ID: "ENG-2", Summary: "Old Summary", Type: "Task", Status: "To Do"},
			},
			payload: core.Manifest{
				Metadata: core.Metadata{Workspace: "eng"},
				Items: []*core.WorkItem{
					{ID: "ENG-2", Summary: "New Summary", Type: "Story", Status: "In Progress"},
				},
			},
			userChoice: 0,
			wantErr:    false,
		},
		{
			name: "Accept Remote Flow (Overwrites Local YAML)",
			seedItems: []*core.WorkItem{
				{ID: "ENG-3", Summary: "Jira Summary Won", Type: "Story", Status: "To Do"},
			},
			payload: core.Manifest{
				Metadata: core.Metadata{Workspace: "eng"},
				Items: []*core.WorkItem{
					{ID: "ENG-3", Summary: "Local Summary Lost", Type: "Story", Status: "To Do"},
				},
			},
			userChoice:        1,
			wantErr:           false,
			checkFileContains: "Jira Summary Won",
		},
		{
			name: "Duplicate ID - Should Skip and Warn",
			payload: core.Manifest{
				Metadata: core.Metadata{Workspace: "eng"},
				Items: []*core.WorkItem{
					{ID: "ENG-100", Summary: "Original", Type: "Story", Status: "To Do"},
					{ID: "ENG-100", Summary: "Duplicate", Type: "Story", Status: "To Do"},
				},
			},
			seedItems: []*core.WorkItem{
				{ID: "ENG-100", Summary: "Original", Type: "Story", Status: "To Do"},
			},
			userChoice: 2,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, factory, ui, inputFile := setupApplyTest(t, tt.payload, tt.seedItems)

			ui.SelectReturn = tt.userChoice
			ui.ReviewDiffReturn = tt.userChoice

			err := commands.Apply(rt, factory, inputFile)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMatch) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errMatch)
			}

			if tt.checkFileContains != "" {
				updatedData, _ := os.ReadFile(inputFile)
				if !strings.Contains(string(updatedData), tt.checkFileContains) {
					t.Errorf("expected updated file to contain %q, got: %s", tt.checkFileContains, string(updatedData))
				}
			}
		})
	}
}
