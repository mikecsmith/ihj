package core

import (
	"sort"
	"strings"
)

// BuildRegistry indexes a flat list of work items by ID.
func BuildRegistry(items []*WorkItem) map[string]*WorkItem {
	reg := make(map[string]*WorkItem, len(items))
	for _, item := range items {
		reg[item.ID] = item
	}
	return reg
}

// LinkChildren wires up parent/child relationships in the registry.
// Children are appended to the parent's Children slice.
func LinkChildren(reg map[string]*WorkItem) {
	// Clear existing children first to avoid duplicates on re-link.
	for _, item := range reg {
		item.Children = nil
	}
	for _, item := range reg {
		if item.ParentID != "" {
			if parent, ok := reg[item.ParentID]; ok {
				parent.Children = append(parent.Children, item)
			}
		}
	}
}

// Roots returns top-level items (those whose parent is not in the registry).
func Roots(reg map[string]*WorkItem) []*WorkItem {
	childIDs := make(map[string]bool)
	for _, item := range reg {
		if item.ParentID != "" {
			if _, ok := reg[item.ParentID]; ok {
				childIDs[item.ID] = true
			}
		}
	}

	roots := make([]*WorkItem, 0, len(reg)-len(childIDs))
	for id, item := range reg {
		if !childIDs[id] {
			roots = append(roots, item)
		}
	}
	return roots
}

// SortItems sorts work items by status weight, type order, then ID.
func SortItems(items []*WorkItem, statusOrder map[string]StatusOrderEntry, typeOrder map[string]TypeOrderEntry) {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aw, bw := statusWeightOf(a.Status, statusOrder), statusWeightOf(b.Status, statusOrder)
		if aw != bw {
			return aw < bw
		}
		ao, bo := typeOrderOf(a.Type, typeOrder), typeOrderOf(b.Type, typeOrder)
		if ao != bo {
			return ao < bo
		}
		return a.ID < b.ID
	})
}

func statusWeightOf(status string, m map[string]StatusOrderEntry) int {
	if e, ok := m[strings.ToLower(status)]; ok {
		return e.Weight
	}
	return 99
}

func typeOrderOf(typeName string, m map[string]TypeOrderEntry) int {
	if e, ok := m[strings.ToLower(typeName)]; ok {
		return e.Order
	}
	return 100
}
