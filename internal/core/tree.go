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
		return compareIDsNatural(a.ID, b.ID)
	})
}

// compareIDsNatural orders IDs so digit runs compare as numbers, making
// "PROJ-9" < "PROJ-10" and "acme/widgets#2" < "acme/widgets#10". Non-digit
// runs compare lexicographically as usual.
func compareIDsNatural(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ad, bd := isDigit(a[i]), isDigit(b[j])
		if ad && bd {
			// Measure and compare digit runs numerically.
			iEnd := i
			for iEnd < len(a) && isDigit(a[iEnd]) {
				iEnd++
			}
			jEnd := j
			for jEnd < len(b) && isDigit(b[jEnd]) {
				jEnd++
			}
			// Strip leading zeros for length-based comparison.
			aRun := trimLeadingZeros(a[i:iEnd])
			bRun := trimLeadingZeros(b[j:jEnd])
			if len(aRun) != len(bRun) {
				return len(aRun) < len(bRun)
			}
			if aRun != bRun {
				return aRun < bRun
			}
			i, j = iEnd, jEnd
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func trimLeadingZeros(s string) string {
	k := 0
	for k < len(s)-1 && s[k] == '0' {
		k++
	}
	return s[k:]
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
