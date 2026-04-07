// convert.go — Jira issue → core.WorkItem conversion and date formatting.

package jira

import (
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

// issuesToWorkItems converts Jira API issues into core.WorkItem values.
// Each WorkItem's Fields map is populated with display-ready values.
// wk provides standard field extraction; customFields maps Jira field IDs
// (e.g. "customfield_10016") to their alias + type binding. Extraction is
// intentionally broad (union of all types); display-time filtering via
// TypeConfig.Fields controls per-type visibility.
func issuesToWorkItems(issues []issue, wk wellKnownFields, customFields map[string]customFieldBinding) []*core.WorkItem {
	items := make([]*core.WorkItem, 0, len(issues))

	for _, iss := range issues {
		f := &iss.Fields

		parentKey := ""
		if f.Parent != nil {
			parentKey = f.Parent.Key
		}

		// Extract standard fields from the well-known field registry.
		fields, displayFields := wk.ExtractFields(f)

		// Extract custom field values using the alias map.
		for fieldID, binding := range customFields {
			alias := binding.Alias
			if alias == "sprint" {
				if val := f.CustomSprint(fieldID); val != "" {
					fields[alias] = val
				}
				continue
			}
			if binding.Type == core.FieldRichText {
				if node := f.CustomRichText(fieldID); node != nil {
					fields[alias] = node
				}
				continue
			}
			if val := f.CustomString(fieldID); val != "" {
				fields[alias] = val
			}
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
func issueToWorkItem(iss *issue, wk wellKnownFields, customFields map[string]customFieldBinding) *core.WorkItem {
	items := issuesToWorkItems([]issue{*iss}, wk, customFields)
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func formatDate(s string) string {
	if len(s) < 10 {
		return ""
	}
	// Return ISO 8601 datetime (YYYY-MM-DDTHH:MM:SS±HH:MM) if available,
	// falling back to date-only (YYYY-MM-DD).
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", s)
	if err != nil {
		return s[:10]
	}
	return t.Format(time.RFC3339)
}

func formatDisplayDate(s string) string {
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
