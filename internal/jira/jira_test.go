package jira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/config"
)

// --- Test fixture helpers ---

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

// --- Registry tests ---

func TestBuildRegistry(t *testing.T) {
	issues := []client.Issue{
		testIssue("FOO-1", "Parent story", "Story", "10", "To Do", "High", ""),
		testIssue("FOO-2", "Child task", "Task", "11", "In Progress", "Medium", "FOO-1"),
	}

	reg := BuildRegistry(issues)

	if len(reg) != 2 {
		t.Fatalf("registry size = %d, want 2", len(reg))
	}

	v := reg["FOO-1"]
	if v.Summary != "Parent story" {
		t.Errorf("summary = %q", v.Summary)
	}
	if v.Type != "Story" {
		t.Errorf("type = %q", v.Type)
	}
	if v.Assignee != "Alice" {
		t.Errorf("assignee = %q", v.Assignee)
	}
	if v.Created != "15 Mar 2024" {
		t.Errorf("created = %q", v.Created)
	}

	child := reg["FOO-2"]
	if child.ParentKey != "FOO-1" {
		t.Errorf("parent = %q", child.ParentKey)
	}
}

func TestBuildRegistry_NilAssignee(t *testing.T) {
	iss := client.Issue{
		Key: "X-1",
		Fields: client.IssueFields{
			Summary:   "test",
			IssueType: client.IssueType{ID: "1", Name: "Task"},
			Status:    client.Status{Name: "Open"},
			Priority:  client.Priority{Name: "Medium"},
			Created:   "2024-01-01T00:00:00.000+0000",
			Updated:   "2024-01-01T00:00:00.000+0000",
		},
	}

	reg := BuildRegistry([]client.Issue{iss})
	if reg["X-1"].Assignee != "Unassigned" {
		t.Errorf("assignee = %q, want 'Unassigned'", reg["X-1"].Assignee)
	}
}

func TestLinkChildren(t *testing.T) {
	issues := []client.Issue{
		testIssue("P-1", "Parent", "Epic", "5", "Open", "High", ""),
		testIssue("P-2", "Child A", "Story", "10", "Open", "Medium", "P-1"),
		testIssue("P-3", "Child B", "Story", "10", "Open", "Low", "P-1"),
		testIssue("P-4", "Orphan", "Task", "11", "Open", "Medium", "MISSING-99"),
	}

	reg := BuildRegistry(issues)
	LinkChildren(reg)

	parent := reg["P-1"]
	if len(parent.Children) != 2 {
		t.Errorf("children count = %d, want 2", len(parent.Children))
	}

	roots := Roots(reg)
	// P-1 and P-4 (orphan parent not in registry) should be roots.
	if len(roots) != 2 {
		t.Errorf("roots = %d, want 2", len(roots))
	}
}

func TestSortViews(t *testing.T) {
	views := []*IssueView{
		{Key: "A-3", Status: "Done", TypeID: "10"},
		{Key: "A-1", Status: "To Do", TypeID: "10"},
		{Key: "A-2", Status: "To Do", TypeID: "5"},
	}

	weights := map[string]int{"to do": 0, "in progress": 1, "done": 2}
	typeOrder := map[string]config.TypeOrderEntry{
		"5":  {Order: 20},
		"10": {Order: 30},
	}

	SortViews(views, weights, typeOrder)

	expected := []string{"A-2", "A-1", "A-3"}
	for i, v := range views {
		if v.Key != expected[i] {
			t.Errorf("position %d: got %s, want %s", i, v.Key, expected[i])
		}
	}
}

// --- Workflow tests ---

func TestFilterTransitions_NoFilter(t *testing.T) {
	transitions := []client.Transition{
		{ID: "1", Name: "To Do"},
		{ID: "2", Name: "In Progress"},
		{ID: "3", Name: "Done"},
	}
	filtered := FilterTransitions(transitions, nil)
	if len(filtered) != 3 {
		t.Errorf("expected all 3, got %d", len(filtered))
	}
}

func TestFilterTransitions_WithAllowed(t *testing.T) {
	transitions := []client.Transition{
		{ID: "1", Name: "To Do"},
		{ID: "2", Name: "In Progress"},
		{ID: "3", Name: "Done"},
		{ID: "4", Name: "Cancelled"},
	}
	filtered := FilterTransitions(transitions, []string{"To Do", "Done"})

	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d", len(filtered))
	}
	if filtered[0].Name != "To Do" || filtered[1].Name != "Done" {
		t.Errorf("wrong order: %v, %v", filtered[0].Name, filtered[1].Name)
	}
}

func TestFilterTransitions_CaseInsensitive(t *testing.T) {
	transitions := []client.Transition{{ID: "1", Name: "In Progress"}}
	filtered := FilterTransitions(transitions, []string{"in progress"})
	if len(filtered) != 1 {
		t.Error("case-insensitive match failed")
	}
}

func TestFindTransitionID(t *testing.T) {
	transitions := []client.Transition{
		{ID: "10", Name: "Start", To: client.Status{Name: "In Progress"}},
		{ID: "20", Name: "Finish", To: client.Status{Name: "Done"}},
	}

	if id := FindTransitionID(transitions, "Start"); id != "10" {
		t.Errorf("by name: got %q", id)
	}
	if id := FindTransitionID(transitions, "Done"); id != "20" {
		t.Errorf("by to.name: got %q", id)
	}
	if id := FindTransitionID(transitions, "Missing"); id != "" {
		t.Errorf("missing: got %q", id)
	}
}

// --- Payload tests ---

func TestBuildSearchRequest(t *testing.T) {
	cf := map[string]string{
		"epic_name_id": "customfield_10009",
		"epic_link_id": "customfield_10008",
	}
	req := BuildSearchRequest("project = FOO", cf, "token123")

	if req.JQL != "project = FOO" {
		t.Errorf("jql = %q", req.JQL)
	}
	if req.NextPageToken != "token123" {
		t.Errorf("token = %q", req.NextPageToken)
	}
	if req.MaxResults != 100 {
		t.Errorf("maxResults = %d", req.MaxResults)
	}

	hasEpicName := false
	for _, f := range req.Fields {
		if f == "customfield_10009" {
			hasEpicName = true
		}
	}
	if !hasEpicName {
		t.Error("missing epic_name custom field in fields")
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
		t.Errorf("summary = %v", fields["summary"])
	}

	issueType, ok := fields["issuetype"].(map[string]any)
	if !ok || issueType["id"] != "10" {
		t.Errorf("issuetype = %v", fields["issuetype"])
	}

	parent, ok := fields["parent"].(map[string]any)
	if !ok || parent["key"] != "FOO-100" {
		t.Errorf("parent = %v (should be uppercased)", fields["parent"])
	}

	if fields["customfield_15000"] != "uuid-abc" {
		t.Errorf("team field = %v, want uuid-abc", fields["customfield_15000"])
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
		t.Error("sub-task should not have team field")
	}
}

// --- Export tests ---

func TestBuildExportHierarchy(t *testing.T) {
	issues := []client.Issue{
		testIssue("E-1", "Epic", "Epic", "5", "Open", "High", ""),
		testIssue("E-2", "Story under epic", "Story", "10", "Open", "Medium", "E-1"),
		testIssue("E-3", "Orphan task", "Task", "11", "Done", "Low", ""),
	}

	roots, hashes := BuildExportHierarchy(issues)

	if len(hashes) != 3 {
		t.Errorf("expected 3 hashes, got %d", len(hashes))
	}

	// Should have 2 roots: E-1 (with E-2 as child) and E-3.
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}

	var epic *ExportIssue
	for _, r := range roots {
		if r.Key == "E-1" {
			epic = r
		}
	}
	if epic == nil {
		t.Fatal("missing E-1 root")
	}
	if len(epic.Children) != 1 || epic.Children[0].Key != "E-2" {
		t.Errorf("epic children = %v", epic.Children)
	}
}

func TestHashDeterministic(t *testing.T) {
	ei := &ExportIssue{Key: "X-1", Type: "Task", Summary: "test", Status: "Open"}
	h1 := hashExportIssue(ei)
	h2 := hashExportIssue(ei)
	if h1 != h2 {
		t.Error("hash not deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 (sha256 hex)", len(h1))
	}
}

// --- Cache tests ---

func TestSaveAndLoadCache(t *testing.T) {
	dir := t.TempDir()
	issues := []client.Issue{
		testIssue("C-1", "Cached", "Task", "1", "Open", "Medium", ""),
	}

	if err := SaveCache(dir, "board", "active", issues); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	loaded, err := LoadCache(dir, "board", "active")
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if len(loaded.Issues) != 1 || loaded.Issues[0].Key != "C-1" {
		t.Errorf("loaded = %v", loaded.Issues)
	}
}

func TestLoadCache_Missing(t *testing.T) {
	_, err := LoadCache(t.TempDir(), "none", "none")
	if err == nil {
		t.Error("expected cache miss error")
	}
}

func TestLoadCache_Corrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad_active.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	_, err := LoadCache(dir, "bad", "active")
	if err == nil {
		t.Error("expected corrupt cache error")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("error = %v, want 'corrupt' mention", err)
	}
}

func TestSaveExportState(t *testing.T) {
	dir := t.TempDir()
	hashes := map[string]string{"X-1": "abc123"}

	if err := SaveExportState(dir, "board", hashes); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, ".board.state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var loaded map[string]string
	if err = json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshaling state file: %v", err)
	}
	if loaded["X-1"] != "abc123" {
		t.Errorf("loaded = %v", loaded)
	}
}

func TestCacheAge_Missing(t *testing.T) {
	age := CacheAge(t.TempDir(), "x", "y")
	if age != -1 {
		t.Errorf("expected -1 for missing cache, got %d", age)
	}
}

// --- Date formatting ---

func TestFormatDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-03-15T10:30:00.000+0000", "15 Mar 2024"},
		{"2024-01-01", "01 Jan 2024"},
		{"short", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatDate(tt.input)
		if got != tt.want {
			t.Errorf("formatDate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2024-03-15T14:30:00.000+0000", "15 Mar 2024, 14:30"},
		{"short", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatDateTime(tt.input)
		if got != tt.want {
			t.Errorf("formatDateTime(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
