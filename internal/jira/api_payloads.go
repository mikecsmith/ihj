package jira

// StandardFields is the field list for search queries. Derived from the
// default well-known field registry (no config-dependent entries like
// sprint/team — those arrive via extraFields as custom fields).
var StandardFields = defaultWellKnownFields().SearchFields()

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
