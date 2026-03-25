package jira

import (
	"sort"
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/document"
)

// IssueView is the processed, renderable representation of an issue.
// Description and comments are ASTs — the TUI/renderer decides how to display.
type IssueView struct {
	Key        string
	Summary    string
	Desc       *document.Node
	Type       string
	TypeID     string
	Status     string
	Priority   string
	Assignee   string
	Reporter   string
	Created    string
	Updated    string
	Labels     string
	Components string
	ParentKey  string
	Comments   []CommentView
	Children   map[string]*IssueView
}

type CommentView struct {
	Author  string
	Created string
	Body    *document.Node
}

// BuildRegistry converts typed API issues into a keyed map of IssueViews.
// All field access goes through the typed IssueFields struct —
// no map[string]any digging.
func BuildRegistry(issues []Issue) map[string]*IssueView {
	reg := make(map[string]*IssueView, len(issues))

	for _, iss := range issues {
		f := &iss.Fields

		var components []string
		for _, c := range f.Components {
			components = append(components, c.Name)
		}

		parentKey := ""
		if f.Parent != nil {
			parentKey = f.Parent.Key
		}

		v := &IssueView{
			Key:        iss.Key,
			Summary:    f.Summary,
			Type:       f.IssueType.Name,
			TypeID:     f.IssueType.ID,
			Status:     f.Status.Name,
			Priority:   f.Priority.Name,
			Assignee:   f.Assignee.DisplayNameOrDefault("Unassigned"),
			Reporter:   f.Reporter.DisplayNameOrDefault("Unassigned"),
			Created:    formatDate(f.Created),
			Updated:    formatDate(f.Updated),
			Labels:     strings.Join(f.Labels, ", "),
			Components: strings.Join(components, ", "),
			ParentKey:  parentKey,
			Children:   make(map[string]*IssueView),
		}

		// Parse ADF description into AST.
		if len(f.Description) > 0 && string(f.Description) != "null" {
			v.Desc, _ = ParseADF(f.Description)
		}

		// Parse last 3 comments.
		if f.Comment != nil && len(f.Comment.Comments) > 0 {
			comments := f.Comment.Comments
			start := max(0, len(comments)-3)
			for _, c := range comments[start:] {
				cv := CommentView{
					Author:  c.Author.DisplayNameOrDefault("Unknown"),
					Created: formatDateTime(c.Created),
				}
				if len(c.Body) > 0 && string(c.Body) != "null" {
					cv.Body, _ = ParseADF(c.Body)
				}
				v.Comments = append(v.Comments, cv)
			}
		}

		reg[iss.Key] = v
	}

	return reg
}

// LinkChildren wires up parent/child relationships in the registry.
func LinkChildren(reg map[string]*IssueView) {
	for key, v := range reg {
		if v.ParentKey != "" {
			if parent, ok := reg[v.ParentKey]; ok {
				parent.Children[key] = v
			}
		}
	}
}

// Roots returns top-level issues (no parent in the registry).
func Roots(reg map[string]*IssueView) []*IssueView {
	childKeys := make(map[string]bool)
	for _, v := range reg {
		if v.ParentKey != "" {
			if _, ok := reg[v.ParentKey]; ok {
				childKeys[v.Key] = true
			}
		}
	}

	roots := make([]*IssueView, 0, len(reg)-len(childKeys))
	for key, v := range reg {
		if !childKeys[key] {
			roots = append(roots, v)
		}
	}
	return roots
}

// SortViews sorts by status weight, type order, then key.
func SortViews(views []*IssueView, statusWeights map[string]int, typeOrder map[string]config.TypeOrderEntry) {
	sort.Slice(views, func(i, j int) bool {
		a, b := views[i], views[j]
		aw, bw := weightOf(a.Status, statusWeights), weightOf(b.Status, statusWeights)
		if aw != bw {
			return aw < bw
		}
		ao, bo := orderOf(a.TypeID, typeOrder), orderOf(b.TypeID, typeOrder)
		if ao != bo {
			return ao < bo
		}
		return a.Key < b.Key
	})
}

func weightOf(status string, m map[string]int) int {
	if w, ok := m[strings.ToLower(status)]; ok {
		return w
	}
	return 99
}

func orderOf(typeID string, m map[string]config.TypeOrderEntry) int {
	if e, ok := m[typeID]; ok {
		return e.Order
	}
	return 100
}

// --- Date formatting ---

func formatDate(s string) string {
	if len(s) < 10 {
		return ""
	}
	t, err := time.Parse("2006-01-02", s[:10])
	if err != nil {
		return s[:10]
	}
	return t.Format("02 Jan 2006")
}

func formatDateTime(s string) string {
	if len(s) < 16 {
		return ""
	}
	t, err := time.Parse("2006-01-02T15:04", s[:16])
	if err != nil {
		return strings.Replace(s[:16], "T", " ", 1)
	}
	return t.Format("02 Jan 2006, 15:04")
}
