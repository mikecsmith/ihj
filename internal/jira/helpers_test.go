package jira


func testIssue(key, summary, typeName, typeID, status, priority string, parentKey string) Issue {
	fields := IssueFields{
		Summary:   summary,
		IssueType: IssueType{ID: typeID, Name: typeName},
		Status:    Status{Name: status, StatusCategory: StatusCategory{Key: "indeterminate"}},
		Priority:  Priority{Name: priority},
		Assignee:  &User{DisplayName: "Alice"},
		Reporter:  &User{DisplayName: "Bob"},
		Labels:    []string{"backend"},
		Created:   "2024-03-15T10:00:00.000+0000",
		Updated:   "2024-03-16T10:00:00.000+0000",
	}
	if parentKey != "" {
		fields.Parent = &ParentRef{Key: parentKey}
	}
	return Issue{Key: key, Fields: fields}
}
