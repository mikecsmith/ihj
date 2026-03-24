package jira

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/config"
)

func TestBuildSearchRequest(t *testing.T) {
	cf := map[string]string{
		"epic_name_id": "customfield_10009",
		"epic_link_id": "customfield_10008",
	}
	req := BuildSearchRequest("project = FOO", cf, "token123")

	if req.JQL != "project = FOO" {
		t.Errorf("BuildSearchRequest().JQL = %q; want \"project = FOO\"", req.JQL)
	}
	if req.NextPageToken != "token123" {
		t.Errorf("BuildSearchRequest().NextPageToken = %q; want \"token123\"", req.NextPageToken)
	}
	if req.MaxResults != 100 {
		t.Errorf("BuildSearchRequest().MaxResults = %d; want 100", req.MaxResults)
	}

	hasEpicName := false
	for _, f := range req.Fields {
		if f == "customfield_10009" {
			hasEpicName = true
		}
	}
	if !hasEpicName {
		t.Error("BuildSearchRequest().Fields does not contain \"customfield_10009\"; want it present")
	}
}

func TestBuildUpsertPayload(t *testing.T) {
	fm := map[string]string{
		"summary":  "Test issue",
		"type":     "Story",
		"priority": "High",
		"parent":   "foo-100",
		"team":     "true",
	}
	types := []config.IssueTypeConfig{
		{ID: 10, Name: "Story"},
		{ID: 11, Name: "Bug"},
	}
	cf := map[string]int{"team": 15000}

	payload := BuildUpsertPayload(fm, map[string]any{"type": "doc"}, types, cf, "FOO", "uuid-abc")

	fields, ok := payload["fields"].(map[string]any)
	if !ok {
		t.Fatal("missing fields")
	}

	if fields["summary"] != "Test issue" {
		t.Errorf("fields[\"summary\"] = %v; want \"Test issue\"", fields["summary"])
	}

	issueType, ok := fields["issuetype"].(map[string]any)
	if !ok || issueType["id"] != "10" {
		t.Errorf("fields[\"issuetype\"] = %v; want map with id=\"10\"", fields["issuetype"])
	}

	parent, ok := fields["parent"].(map[string]any)
	if !ok || parent["key"] != "FOO-100" {
		t.Errorf("fields[\"parent\"] = %v; want map with key=\"FOO-100\" (uppercased)", fields["parent"])
	}

	if fields["customfield_15000"] != "uuid-abc" {
		t.Errorf("fields[\"customfield_15000\"] = %v; want \"uuid-abc\"", fields["customfield_15000"])
	}
}

func TestBuildUpsertPayload_SubtaskSkipsTeam(t *testing.T) {
	fm := map[string]string{
		"summary": "Sub",
		"type":    "Sub-task",
		"team":    "true",
	}
	types := []config.IssueTypeConfig{{ID: 20, Name: "Sub-task"}}
	cf := map[string]int{"team": 15000}

	payload := BuildUpsertPayload(fm, nil, types, cf, "FOO", "uuid")
	fields := payload["fields"].(map[string]any)

	if _, ok := fields["customfield_15000"]; ok {
		t.Error("fields[\"customfield_15000\"] exists; want absent for sub-task")
	}
}
