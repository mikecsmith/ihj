package core_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestBuildRegistry(t *testing.T) {
	items := []*core.WorkItem{
		{ID: "A-1", Summary: "First"},
		{ID: "A-2", Summary: "Second"},
	}
	reg := core.BuildRegistry(items)

	if len(reg) != 2 {
		t.Fatalf("len(reg) = %d; want 2", len(reg))
	}
	if reg["A-1"].Summary != "First" {
		t.Errorf("reg[A-1].Summary = %q; want \"First\"", reg["A-1"].Summary)
	}
}

func TestLinkChildren_BasicParenting(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent"}
	child := &core.WorkItem{ID: "C-1", Summary: "Child", ParentID: "P-1"}
	reg := map[string]*core.WorkItem{"P-1": parent, "C-1": child}

	core.LinkChildren(reg)

	if len(parent.Children) != 1 || parent.Children[0].ID != "C-1" {
		t.Errorf("parent.Children = %v; want [C-1]", ids(parent.Children))
	}
}

func TestLinkChildren_Idempotent(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent"}
	child := &core.WorkItem{ID: "C-1", Summary: "Child", ParentID: "P-1"}
	reg := map[string]*core.WorkItem{"P-1": parent, "C-1": child}

	core.LinkChildren(reg)
	core.LinkChildren(reg)

	if len(parent.Children) != 1 {
		t.Errorf("after double LinkChildren, parent.Children = %d; want 1", len(parent.Children))
	}
}

func TestLinkChildren_MissingParentIgnored(t *testing.T) {
	orphan := &core.WorkItem{ID: "O-1", Summary: "Orphan", ParentID: "GONE-99"}
	reg := map[string]*core.WorkItem{"O-1": orphan}

	core.LinkChildren(reg)

	// Orphan should not crash and should remain in registry.
	if _, ok := reg["O-1"]; !ok {
		t.Error("orphan should remain in registry")
	}
}

func TestLinkChildren_ClearsStaleChildren(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent"}
	child := &core.WorkItem{ID: "C-1", Summary: "Child", ParentID: "P-1"}
	reg := map[string]*core.WorkItem{"P-1": parent, "C-1": child}

	core.LinkChildren(reg)
	if len(parent.Children) != 1 {
		t.Fatalf("setup: parent.Children = %d; want 1", len(parent.Children))
	}

	// Remove the child's parent relationship and re-link.
	child.ParentID = ""
	core.LinkChildren(reg)

	if len(parent.Children) != 0 {
		t.Errorf("after removing parent, parent.Children = %d; want 0", len(parent.Children))
	}
}

func TestRoots_ExcludesChildrenWithParentInRegistry(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent"}
	child := &core.WorkItem{ID: "C-1", Summary: "Child", ParentID: "P-1"}
	reg := map[string]*core.WorkItem{"P-1": parent, "C-1": child}

	roots := core.Roots(reg)

	if len(roots) != 1 || roots[0].ID != "P-1" {
		t.Errorf("Roots() = %v; want [P-1]", ids(roots))
	}
}

func TestRoots_OrphanParentTreatedAsRoot(t *testing.T) {
	// ParentID references an issue not in the registry — treated as root.
	orphan := &core.WorkItem{ID: "O-1", Summary: "Orphan", ParentID: "MISSING-1"}
	reg := map[string]*core.WorkItem{"O-1": orphan}

	roots := core.Roots(reg)

	if len(roots) != 1 || roots[0].ID != "O-1" {
		t.Errorf("Roots() = %v; want [O-1]", ids(roots))
	}
}

func TestRoots_AllRootsWhenNoParents(t *testing.T) {
	reg := map[string]*core.WorkItem{
		"A-1": {ID: "A-1", Summary: "A"},
		"A-2": {ID: "A-2", Summary: "B"},
	}

	roots := core.Roots(reg)

	if len(roots) != 2 {
		t.Errorf("Roots() len = %d; want 2", len(roots))
	}
}

func TestLinkChildren_MultipleChildrenLinked(t *testing.T) {
	parent := &core.WorkItem{ID: "P-1", Summary: "Parent"}
	c1 := &core.WorkItem{ID: "C-1", Summary: "Child 1", ParentID: "P-1"}
	c2 := &core.WorkItem{ID: "C-2", Summary: "Child 2", ParentID: "P-1"}
	reg := map[string]*core.WorkItem{"P-1": parent, "C-1": c1, "C-2": c2}

	core.LinkChildren(reg)

	if len(parent.Children) != 2 {
		t.Errorf("parent.Children = %d; want 2", len(parent.Children))
	}
}

func TestLinkChildren_ReparentMovesChild(t *testing.T) {
	oldParent := &core.WorkItem{ID: "OLD-1", Summary: "Old Parent"}
	newParent := &core.WorkItem{ID: "NEW-1", Summary: "New Parent"}
	child := &core.WorkItem{ID: "C-1", Summary: "Child", ParentID: "OLD-1"}
	reg := map[string]*core.WorkItem{
		"OLD-1": oldParent, "NEW-1": newParent, "C-1": child,
	}

	core.LinkChildren(reg)
	if len(oldParent.Children) != 1 {
		t.Fatal("setup: child not linked to old parent")
	}

	// Re-parent the child.
	child.ParentID = "NEW-1"
	core.LinkChildren(reg)

	if len(oldParent.Children) != 0 {
		t.Errorf("old parent still has %d children; want 0", len(oldParent.Children))
	}
	if len(newParent.Children) != 1 || newParent.Children[0].ID != "C-1" {
		t.Errorf("new parent children = %v; want [C-1]", ids(newParent.Children))
	}
}

func ids(items []*core.WorkItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.ID
	}
	return out
}
