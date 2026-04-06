// Package core defines the universal domain model for work orchestration.
//
// It abstracts backend-specific concepts (like Jira issues, Trello cards,
// or GitHub issues) into a standardized WorkItem structure. This allows
// the core application to validate, diff, and manipulate hierarchies of
// tasks without needing to understand the underlying tracking provider.
package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mikecsmith/ihj/internal/document"
)

// WorkItem represents a universal unit of work (Issue, Card, Task, etc.)
type WorkItem struct {
	ID       string `json:"id" yaml:"-"`
	Type     string `json:"type" yaml:"-"`
	Summary  string `json:"summary" yaml:"-"`
	Status   string `json:"status" yaml:"-"`
	ParentID string `json:"parentId" yaml:"-"`

	// DisplayID is the short form of ID used by the TUI where space is
	// tight (list column, detail header). Empty means "use ID".
	DisplayID string `json:"-" yaml:"-"`

	// Location is a per-item scope hint the TUI surfaces in the detail
	// breadcrumb (e.g. "acme/widgets" for a GitHub issue living in a
	// specific repo). Empty means "fall back to workspace name".
	Location string `json:"-" yaml:"-"`

	// Indicators is a pre-rendered string of icon glyphs (emoji or
	// unicode symbols) the TUI shows in the list's priority cell to
	// summarise labels/flags at a glance. Providers populate it;
	// empty means "fall back to the workspace's primary urgency field".
	Indicators string `json:"-" yaml:"-"`

	// Description is the AST representation — the interchange format.
	// Manifest serialization lives in the encoding package.
	Description *document.Node `json:"-" yaml:"-"`

	// Comments on this work item.
	Comments []Comment `json:"-" yaml:"-"`

	// Fields holds arbitrary backend-specific data (Priority, Sprint, Team, etc.)
	Fields map[string]any `json:"fields" yaml:"-"`

	// DisplayFields holds display-only values (e.g., user display names)
	// that should appear in the UI but never in exports or diffs.
	DisplayFields map[string]any `json:"displayFields" yaml:"-"`

	// DecodedKeys records which keys were explicitly present in the source
	// payload (manifest YAML, frontmatter, etc.). Populated by decoders;
	// nil means "not tracked" — callers should infer presence from values.
	DecodedKeys SetKeys `json:"-" yaml:"-"`

	Children []*WorkItem `json:"children" yaml:"-"`
}

// Field accessors for common Fields entries.

func (w *WorkItem) StringField(key string) string {
	if v, ok := w.Fields[key].(string); ok {
		return v
	}
	return ""
}

// DisplayStringField returns the display-friendly value for a field.
// It checks DisplayFields first, then falls back to Fields.
// String slices are joined with ", " for display.
func (w *WorkItem) DisplayStringField(key string) string {
	if v, ok := w.DisplayFields[key].(string); ok && v != "" {
		return v
	}
	if v, ok := w.Fields[key].([]string); ok && len(v) > 0 {
		return strings.Join(v, ", ")
	}
	return w.StringField(key)
}

// RichTextField returns a rich-text field value as a document.Node AST.
// Returns nil if the key is not present or not a *document.Node.
func (w *WorkItem) RichTextField(key string) *document.Node {
	if v, ok := w.Fields[key].(*document.Node); ok {
		return v
	}
	return nil
}

func (w *WorkItem) StringSliceField(key string) []string {
	if v, ok := w.Fields[key].([]string); ok {
		return v
	}
	return nil
}

// DescriptionMarkdown returns the description rendered as markdown text.
// Returns empty string if Description is nil.
func (w *WorkItem) DescriptionMarkdown() string {
	return RenderRichText(w.Description)
}

// RenderRichText renders a rich-text field value to trimmed Markdown. Accepts
// *document.Node values; nil and non-Node values yield empty string. Used to
// produce stable, comparable representations of RichText fields.
func RenderRichText(v any) string {
	node, ok := v.(*document.Node)
	if !ok || node == nil {
		return ""
	}
	return strings.TrimSpace(document.RenderMarkdown(node))
}

// IsZeroFieldValue reports whether a field value is considered empty.
func IsZeroFieldValue(v any) bool {
	switch val := v.(type) {
	case string:
		return val == ""
	case []string:
		return len(val) == 0
	case []any:
		return len(val) == 0
	case bool:
		return !val
	case nil:
		return true
	default:
		return false
	}
}

// ContentHash generates a hash of the item's core data and flex fields.
// This is used during export and diffing to detect changes.
func (w *WorkItem) ContentHash() string {
	payload := map[string]any{
		"id":           w.ID,
		KeyType:        w.Type,
		KeySummary:     w.Summary,
		KeyStatus:      w.Status,
		KeyDescription: w.DescriptionMarkdown(),
		KeyFields:      w.Fields,
	}

	data, _ := json.Marshal(payload)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

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
