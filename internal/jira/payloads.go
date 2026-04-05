package jira

// StandardFields is the field list for search queries.
var StandardFields = []string{
	"summary", "issuetype", "status", "priority", "parent",
	"subtasks", "description", "assignee", "comment", "reporter",
	"created", "updated", "labels", "components",
}

// buildSearchRequest constructs the search API request body.
// extraFields are additional Jira field IDs (e.g. "customfield_10016")
// to include in the response, typically derived from createmeta.
func buildSearchRequest(jql string, formattedCF map[string]string, extraFields []string, nextToken string) searchRequest {
	fields := make([]string, len(StandardFields))
	copy(fields, StandardFields)

	if id, ok := formattedCF["epic_name_id"]; ok {
		fields = append(fields, id)
	}
	if id, ok := formattedCF["epic_link_id"]; ok {
		fields = append(fields, id)
	}

	fields = append(fields, extraFields...)

	return searchRequest{
		JQL:           jql,
		Fields:        fields,
		MaxResults:    100,
		NextPageToken: nextToken,
	}
}
