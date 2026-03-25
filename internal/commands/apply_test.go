package commands_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/work"
)

// setupApplyTest scaffolds the test environment using only public APIs.
func setupApplyTest(t *testing.T, payload work.Manifest, seedIssues []client.Issue) (*commands.App, *commands.MockUI, string) {
	t.Helper()

	dir := t.TempDir()

	// Keep the cache internal to the test run
	cacheDir := filepath.Join(dir, "cache")
	_ = os.MkdirAll(cacheDir, 0o755)

	inputFile := filepath.Join(dir, "import.json")
	data, _ := json.Marshal(payload)
	if err := os.WriteFile(inputFile, data, 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	mockUI := &commands.MockUI{}
	app := commands.NewTestApp(mockUI)
	app.CacheDir = cacheDir

	// Initialize the mock client with seed data
	mockClient := client.NewMockClient(seedIssues, []string{"To Do", "In Progress", "Done"}, "ENG")
	app.Client = mockClient

	return app, mockUI, inputFile
}

func TestApply(t *testing.T) {
	tests := []struct {
		name              string
		payload           work.Manifest
		seedIssues        []client.Issue
		userChoice        int // 0 = Apply/Create, 1 = Accept Remote, 2 = Skip, 3 = Abort
		wantErr           bool
		errMatch          string
		checkFileContains string // If set, asserts this string exists in the saved file
	}{
		{
			name: "Validation Failure - Invalid Type",
			payload: work.Manifest{
				Metadata: work.Metadata{Backend: "jira", Target: "eng"},
				Items: []*work.WorkItem{
					{Summary: "Invalid", Type: "MagicType", Status: "To Do"},
				},
			},
			wantErr:  true,
			errMatch: "validation failed",
		},
		{
			name: "Successful Creation Flow",
			payload: work.Manifest{
				Metadata: work.Metadata{Backend: "jira", Target: "eng"},
				Items: []*work.WorkItem{
					{Summary: "New Story", Type: "Story", Status: "To Do"},
				},
			},
			userChoice:        0,      // Create (Index 0 in Select)
			checkFileContains: "ENG-", // Ensure the new ID was saved to the file
		},
		{
			name: "Idempotency - No Changes Found",
			seedIssues: []client.Issue{
				{
					Key: "ENG-1",
					Fields: client.IssueFields{
						Summary:   "Same",
						IssueType: client.IssueType{Name: "Story"},
						Status:    client.Status{Name: "To Do"},
					},
				},
			},
			payload: work.Manifest{
				Metadata: work.Metadata{Backend: "jira", Target: "eng"},
				Items: []*work.WorkItem{
					{ID: "ENG-1", Summary: "Same", Type: "Story", Status: "To Do"},
				},
			},
			wantErr: false,
		},
		{
			name: "Successful Update Flow (Apply to Jira)",
			seedIssues: []client.Issue{
				{
					Key: "ENG-2",
					Fields: client.IssueFields{
						Summary:   "Old Summary",
						IssueType: client.IssueType{Name: "Task"},
						Status:    client.Status{Name: "To Do"},
					},
				},
			},
			payload: work.Manifest{
				Metadata: work.Metadata{Backend: "jira", Target: "eng"},
				Items: []*work.WorkItem{
					{ID: "ENG-2", Summary: "New Summary", Type: "Story", Status: "In Progress"},
				},
			},
			userChoice: 0, // Apply to Jira
			wantErr:    false,
		},
		{
			name: "Accept Remote Flow (Overwrites Local YAML)",
			seedIssues: []client.Issue{
				{
					Key: "ENG-3",
					Fields: client.IssueFields{
						Summary:   "Jira Summary Won",
						IssueType: client.IssueType{Name: "Story"},
						Status:    client.Status{Name: "To Do"},
					},
				},
			},
			payload: work.Manifest{
				Metadata: work.Metadata{Backend: "jira", Target: "eng"},
				Items: []*work.WorkItem{
					// Local YAML has old data, we want to accept the remote Jira state
					{ID: "ENG-3", Summary: "Local Summary Lost", Type: "Story", Status: "To Do"},
				},
			},
			userChoice:        1, // Accept Remote (Update Local)
			wantErr:           false,
			checkFileContains: "Jira Summary Won", // Asserts the YAML was successfully overwritten
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, mockUI, inputFile := setupApplyTest(t, tt.payload, tt.seedIssues)

			mockUI.SelectReturn = tt.userChoice
			mockUI.ReviewDiffReturn = tt.userChoice

			err := commands.Apply(app, inputFile)

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
