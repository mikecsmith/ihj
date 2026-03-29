package jira

import (
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

// issuesToWorkItems converts Jira API issues into core.WorkItem values.
// Each WorkItem's Fields map is populated with display-ready values.
func issuesToWorkItems(issues []issue) []*core.WorkItem {
	items := make([]*core.WorkItem, 0, len(issues))

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

		fields := map[string]any{
			"priority": f.Priority.Name,
			"assignee": f.Assignee.EmailOrDefault(""),
			"reporter": f.Reporter.EmailOrDefault(""),
			"created":  formatDate(f.Created),
			"updated":  formatDate(f.Updated),
		}

		displayFields := map[string]any{
			"assignee": f.Assignee.DisplayNameOrDefault(""),
			"reporter": f.Reporter.DisplayNameOrDefault(""),
		}
		if len(f.Labels) > 0 {
			fields["labels"] = f.Labels
		}
		if len(components) > 0 {
			fields["components"] = components
		}

		item := &core.WorkItem{
			ID:            iss.Key,
			Summary:       f.Summary,
			Type:          f.IssueType.Name,
			Status:        f.Status.Name,
			ParentID:      parentKey,
			Fields:        fields,
			DisplayFields: displayFields,
		}

		// Parse ADF description into AST.
		if len(f.Description) > 0 && string(f.Description) != "null" {
			item.Description, _ = parseADF(f.Description)
		}

		// Parse last 3 comments.
		if f.Comment != nil && len(f.Comment.Comments) > 0 {
			comments := f.Comment.Comments
			start := max(0, len(comments)-3)
			for _, c := range comments[start:] {
				cv := core.Comment{
					Author:  c.Author.DisplayNameOrDefault("Unknown"),
					Created: formatDateTime(c.Created),
				}
				if len(c.Body) > 0 && string(c.Body) != "null" {
					cv.Body, _ = parseADF(c.Body)
				}
				item.Comments = append(item.Comments, cv)
			}
		}

		items = append(items, item)
	}

	return items
}

// issueToWorkItem converts a single Jira issue to a core.WorkItem.
func issueToWorkItem(iss *issue) *core.WorkItem {
	items := issuesToWorkItems([]issue{*iss})
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func formatDate(s string) string {
	if len(s) < 10 {
		return ""
	}
	// Return ISO 8601 date (YYYY-MM-DD).
	return s[:10]
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
