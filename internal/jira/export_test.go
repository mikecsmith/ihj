package jira

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/work"
)

func TestBuildExportHierarchy(t *testing.T) {
	issues := []client.Issue{
		testIssue("E-1", "Epic", "Epic", "5", "Open", "High", ""),
		testIssue("E-2", "Story under epic", "Story", "10", "Open", "Medium", "E-1"),
		testIssue("E-3", "Orphan task", "Task", "11", "Done", "Low", ""),
	}

	roots, hashes := BuildExportHierarchy(issues)

	if len(hashes) != 3 {
		t.Errorf("len(hashes) = %d; want 3", len(hashes))
	}

	// Should have 2 roots: E-1 (with E-2 as child) and E-3.
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}

	var epic *work.WorkItem
	for _, r := range roots {
		if r.ID == "E-1" {
			epic = r
		}
	}
	if epic == nil {
		t.Fatal("missing E-1 root")
	}
	if len(epic.Children) != 1 || epic.Children[0].ID != "E-2" {
		t.Errorf("epic.Children = %v; want 1 child with Key=\"E-2\"", epic.Children)
	}
}
