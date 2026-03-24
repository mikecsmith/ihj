package jira

import "github.com/mikecsmith/ihj/internal/client"

func testIssue(key, summary, typeName, typeID, status, priority string, parentKey string) client.Issue {
	fields := client.IssueFields{
		Summary:   summary,
		IssueType: client.IssueType{ID: typeID, Name: typeName},
		Status:    client.Status{Name: status, StatusCategory: client.StatusCategory{Key: "indeterminate"}},
		Priority:  client.Priority{Name: priority},
		Assignee:  &client.User{DisplayName: "Alice"},
		Reporter:  &client.User{DisplayName: "Bob"},
		Labels:    []string{"backend"},
		Created:   "2024-03-15T10:00:00.000+0000",
		Updated:   "2024-03-16T10:00:00.000+0000",
	}
	if parentKey != "" {
		fields.Parent = &client.ParentRef{Key: parentKey}
	}
	return client.Issue{Key: key, Fields: fields}
}
