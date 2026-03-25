package jira

import (
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/config"
)

// StandardFields is the field list for search queries.
var StandardFields = []string{
	"summary", "issuetype", "status", "priority", "parent",
	"subtasks", "description", "assignee", "comment", "reporter",
	"created", "updated", "labels", "components",
}

// BuildSearchRequest constructs the search API request body.
func BuildSearchRequest(jql string, formattedCF map[string]string, nextToken string) SearchRequest {
	fields := make([]string, len(StandardFields))
	copy(fields, StandardFields)

	if id, ok := formattedCF["epic_name_id"]; ok {
		fields = append(fields, id)
	}
	if id, ok := formattedCF["epic_link_id"]; ok {
		fields = append(fields, id)
	}

	return SearchRequest{
		JQL:           jql,
		Fields:        fields,
		MaxResults:    100,
		NextPageToken: nextToken,
	}
}

// BuildUpsertPayload constructs the POST/PUT body from parsed frontmatter.
func BuildUpsertPayload(
	fm map[string]string,
	adfDescription map[string]any,
	types []config.IssueTypeConfig,
	customFields map[string]int,
	projectKey, teamUUID string,
) map[string]any {
	fields := map[string]any{
		"summary":     fm["summary"],
		"description": adfDescription,
	}

	typeName := fm["type"]
	for _, t := range types {
		if t.Name == typeName {
			fields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", t.ID)}
			break
		}
	}

	isSubtask := strings.EqualFold(typeName, "sub-task")

	if parent := fm["parent"]; parent != "" {
		fields["parent"] = map[string]any{"key": strings.ToUpper(parent)}
	}
	if priority := fm["priority"]; priority != "" {
		fields["priority"] = map[string]any{"name": priority}
	}

	for cfName, cfID := range customFields {
		val := fm[cfName]
		if val == "" || (cfName == "team" && isSubtask) {
			continue
		}
		fieldKey := fmt.Sprintf("customfield_%d", cfID)
		if cfName == "team" && strings.EqualFold(val, "true") && teamUUID != "" {
			fields[fieldKey] = teamUUID
		} else if cfName != "team" {
			fields[fieldKey] = val
		}
	}

	if projectKey != "" {
		fields["project"] = map[string]any{"key": projectKey}
	}

	return map[string]any{"fields": fields}
}
