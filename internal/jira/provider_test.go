package jira_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
)

// testWorkspace builds a minimal workspace with a Jira config suitable for
// creating a Provider. serverURL is injected from httptest.
func testWorkspace(serverURL string) *core.Workspace {
	ws := &core.Workspace{
		Slug:     "test",
		Name:     "Test",
		Provider: core.ProviderJira,
		BaseURL:  serverURL,
		CacheTTL: core.DefaultCacheTTL,
		Types: []core.TypeConfig{
			{ID: 10, Name: "Story", Order: 1},
			{ID: 11, Name: "Task", Order: 2},
			{ID: 12, Name: "Bug", Order: 3},
		},
		Statuses: []core.StatusConfig{{Name: "To Do", Order: 10, Color: "default"}, {Name: "In Progress", Order: 20, Color: "default"}, {Name: "Done", Order: 30, Color: "green"}},
		Filters:  map[string]string{"active": `status != "Done"`},
		ProviderConfig: map[string]any{
			"server":      serverURL,
			"project_key": "FOO",
			"jql":         `project = "{project_key}"`,
			"board_id":    float64(42),
			"custom_fields": map[string]any{
				"team": float64(15000),
			},
			"team_uuid": "uuid-abc",
		},
	}
	return ws
}

// newTestProvider creates a Provider backed by an httptest server.
// Returns the provider and a cleanup function.
func newTestProvider(t *testing.T, handler http.Handler) (*jira.Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	ws := testWorkspace(srv.URL)
	cfg, err := jira.HydrateWorkspace(ws)
	if err != nil {
		t.Fatalf("HydrateWorkspace: %v", err)
	}
	_ = cfg

	client := jira.New(srv.URL, "test-token")
	provider := jira.NewProvider(client, ws, t.TempDir())
	return provider, srv
}

// issueJSON builds a minimal Jira issue JSON string.
func issueJSON(key, summary, typeName, typeID, statusName string) string {
	return `{
		"key": "` + key + `",
		"id": "100",
		"fields": {
			"summary": "` + summary + `",
			"issuetype": {"id": "` + typeID + `", "name": "` + typeName + `"},
			"status": {"id": "1", "name": "` + statusName + `", "statusCategory": {"id": 2, "key": "indeterminate"}},
			"priority": {"id": "3", "name": "Medium"},
			"assignee": {"accountId": "u1", "displayName": "Alice", "emailAddress": "alice@example.com"},
			"reporter": {"accountId": "u2", "displayName": "Bob", "emailAddress": "bob@example.com"},
			"labels": ["backend"],
			"components": [],
			"created": "2024-03-15T10:00:00.000+0000",
			"updated": "2024-03-16T10:00:00.000+0000"
		}
	}`
}

func TestProvider_Search(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"issues": []json.RawMessage{
					json.RawMessage(issueJSON("FOO-1", "First story", "Story", "10", "To Do")),
					json.RawMessage(issueJSON("FOO-2", "Second task", "Task", "11", "In Progress")),
				},
				"total":  2,
				"isLast": true,
			})
			return
		}
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)

	items, err := provider.Search(context.Background(), "", true)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Search() returned %d items; want 2", len(items))
	}
	if items[0].ID != "FOO-1" {
		t.Errorf("items[0].ID = %q; want \"FOO-1\"", items[0].ID)
	}
	if items[0].Summary != "First story" {
		t.Errorf("items[0].Summary = %q; want \"First story\"", items[0].Summary)
	}
	if items[0].Type != "Story" {
		t.Errorf("items[0].Type = %q; want \"Story\"", items[0].Type)
	}
	if items[1].Status != "In Progress" {
		t.Errorf("items[1].Status = %q; want \"In Progress\"", items[1].Status)
	}
}

func TestProvider_Search_WithFilter(t *testing.T) {
	var receivedJQL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			receivedJQL, _ = body["jql"].(string)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"issues": []json.RawMessage{},
				"total":  0,
				"isLast": true,
			})
			return
		}
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)

	_, err := provider.Search(context.Background(), "active", true)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// The filter "active" → status != "Done" should be combined with the base JQL.
	if !strings.Contains(receivedJQL, `project = "FOO"`) {
		t.Errorf("JQL = %q; want containing base project clause", receivedJQL)
	}
	if !strings.Contains(receivedJQL, `status != "Done"`) {
		t.Errorf("JQL = %q; want containing active filter clause", receivedJQL)
	}
}

func TestProvider_Search_UsesCache(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"issues": []json.RawMessage{
					json.RawMessage(issueJSON("FOO-1", "Cached", "Story", "10", "To Do")),
				},
				"total":  1,
				"isLast": true,
			})
			return
		}
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)
	ctx := context.Background()

	// First call populates cache.
	items, err := provider.Search(ctx, "", true)
	if err != nil {
		t.Fatalf("Search(noCache=true) error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Search() returned %d items; want 1", len(items))
	}

	// Second call should use cache (noCache=false).
	items2, err := provider.Search(ctx, "", false)
	if err != nil {
		t.Fatalf("Search(noCache=false) error = %v", err)
	}
	if len(items2) != 1 {
		t.Fatalf("cached Search() returned %d items; want 1", len(items2))
	}
	if callCount != 1 {
		t.Errorf("API called %d times; want 1 (second call should use cache)", callCount)
	}
}

func TestProvider_Get(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/FOO-1" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(issueJSON("FOO-1", "Detail view", "Story", "10", "In Progress")))
			return
		}
		w.WriteHeader(404)
		w.Write([]byte(`{"errorMessages":["Issue not found"]}`))
	})

	provider, _ := newTestProvider(t, handler)

	item, err := provider.Get(context.Background(), "FOO-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if item.ID != "FOO-1" {
		t.Errorf("ID = %q; want \"FOO-1\"", item.ID)
	}
	if item.Summary != "Detail view" {
		t.Errorf("Summary = %q; want \"Detail view\"", item.Summary)
	}
	if item.StringField("assignee") != "alice@example.com" {
		t.Errorf("assignee = %q; want \"alice@example.com\"", item.StringField("assignee"))
	}
}

func TestProvider_Get_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"errorMessages":["Issue not found"]}`))
	})

	provider, _ := newTestProvider(t, handler)

	_, err := provider.Get(context.Background(), "MISSING-1")
	if err == nil {
		t.Fatal("Get() error = nil; want error for missing issue")
	}
}

func TestProvider_Create(t *testing.T) {
	var receivedPayload map[string]any
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedPayload)
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{
				"id":   "10001",
				"key":  "FOO-99",
				"self": "https://x/10001",
			})
			return
		}
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)

	key, err := provider.Create(context.Background(), &core.WorkItem{
		Summary: "New story",
		Type:    "Story",
		Status:  "To Do",
		Fields:  map[string]any{"priority": "High"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if key != "FOO-99" {
		t.Errorf("Create() key = %q; want \"FOO-99\"", key)
	}

	// Verify the payload was correctly built.
	fields, ok := receivedPayload["fields"].(map[string]any)
	if !ok {
		t.Fatal("payload missing fields")
	}
	if fields["summary"] != "New story" {
		t.Errorf("fields.summary = %v; want \"New story\"", fields["summary"])
	}
}

func TestProvider_Update_Summary(t *testing.T) {
	var receivedPayload map[string]any
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/FOO-1" && r.Method == "PUT" {
			json.NewDecoder(r.Body).Decode(&receivedPayload)
			w.WriteHeader(204)
			return
		}
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)

	newSummary := "Updated summary"
	err := provider.Update(context.Background(), "FOO-1", &core.Changes{
		Summary: &newSummary,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	fields := receivedPayload["fields"].(map[string]any)
	if fields["summary"] != "Updated summary" {
		t.Errorf("fields.summary = %v; want \"Updated summary\"", fields["summary"])
	}
}

func TestProvider_Update_StatusTransition(t *testing.T) {
	transitionCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/issue/FOO-1/transitions" && r.Method == "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"transitions": []map[string]any{
					{"id": "31", "name": "Start", "to": map[string]any{"name": "In Progress"}},
					{"id": "41", "name": "Done", "to": map[string]any{"name": "Done"}},
				},
			})
		case r.URL.Path == "/rest/api/3/issue/FOO-1/transitions" && r.Method == "POST":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			tr := body["transition"].(map[string]any)
			if tr["id"] != "31" {
				t.Errorf("transition id = %v; want \"31\"", tr["id"])
			}
			transitionCalled = true
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	})

	provider, _ := newTestProvider(t, handler)

	newStatus := "In Progress"
	err := provider.Update(context.Background(), "FOO-1", &core.Changes{
		Status: &newStatus,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !transitionCalled {
		t.Error("expected transition API to be called")
	}
}

func TestProvider_Comment(t *testing.T) {
	var receivedBody map[string]any
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/FOO-1/comment" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"id": "500"})
			return
		}
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)

	err := provider.Comment(context.Background(), "FOO-1", "This is a **test** comment")
	if err != nil {
		t.Fatalf("Comment() error = %v", err)
	}

	// The body should be an ADF document.
	body, ok := receivedBody["body"].(map[string]any)
	if !ok {
		t.Fatal("comment body missing or not an ADF document")
	}
	if body["type"] != "doc" {
		t.Errorf("body.type = %v; want \"doc\"", body["type"])
	}
}

func TestProvider_Assign(t *testing.T) {
	var assignedTo string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/myself":
			json.NewEncoder(w).Encode(map[string]any{
				"accountId":   "user-123",
				"displayName": "Alice",
			})
		case r.URL.Path == "/rest/api/3/issue/FOO-1/assignee" && r.Method == "PUT":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			assignedTo, _ = body["accountId"].(string)
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	})

	provider, _ := newTestProvider(t, handler)

	err := provider.Assign(context.Background(), "FOO-1")
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}
	if assignedTo != "user-123" {
		t.Errorf("assigned to = %q; want \"user-123\"", assignedTo)
	}
}

func TestProvider_CurrentUser(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" {
			json.NewEncoder(w).Encode(map[string]any{
				"accountId":    "user-123",
				"displayName":  "Alice",
				"emailAddress": "alice@example.com",
			})
			return
		}
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)

	u, err := provider.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if u.ID != "user-123" {
		t.Errorf("ID = %q; want \"user-123\"", u.ID)
	}
	if u.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q; want \"Alice\"", u.DisplayName)
	}
}

func TestProvider_Capabilities(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)
	caps := provider.Capabilities()

	if !caps.HasTransitions {
		t.Error("HasTransitions = false; want true")
	}
	if !caps.HasHierarchy {
		t.Error("HasHierarchy = false; want true")
	}
}

func TestProvider_ContentRenderer_Roundtrip(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)
	renderer := provider.ContentRenderer()

	// Build an ADF document as raw JSON (what Jira would return).
	adfMap := map[string]any{
		"version": float64(1),
		"type":    "doc",
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "Hello ",
					},
					map[string]any{
						"type": "text",
						"text": "bold",
						"marks": []any{
							map[string]any{"type": "strong"},
						},
					},
				},
			},
		},
	}

	// Parse ADF → AST.
	node, err := renderer.ParseContent(adfMap)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}
	if node.Type != document.NodeDoc {
		t.Fatalf("node.Type = %v; want NodeDoc", node.Type)
	}
	if len(node.Children) != 1 {
		t.Fatalf("len(Children) = %d; want 1", len(node.Children))
	}

	// Render AST → ADF.
	rendered, err := renderer.RenderContent(node)
	if err != nil {
		t.Fatalf("RenderContent() error = %v", err)
	}
	adfOut, ok := rendered.(map[string]any)
	if !ok {
		t.Fatalf("RenderContent() type = %T; want map[string]any", rendered)
	}
	if adfOut["type"] != "doc" {
		t.Errorf("rendered type = %v; want \"doc\"", adfOut["type"])
	}

	// Re-parse to verify roundtrip fidelity.
	node2, err := renderer.ParseContent(adfOut)
	if err != nil {
		t.Fatalf("re-ParseContent() error = %v", err)
	}
	if len(node2.Children) != 1 {
		t.Fatalf("roundtrip Children = %d; want 1", len(node2.Children))
	}
	p := node2.Children[0]
	if len(p.Children) != 2 {
		t.Fatalf("paragraph Children = %d; want 2", len(p.Children))
	}
	if p.Children[0].Text != "Hello " {
		t.Errorf("text[0] = %q; want \"Hello \"", p.Children[0].Text)
	}
	if p.Children[1].Text != "bold" {
		t.Errorf("text[1] = %q; want \"bold\"", p.Children[1].Text)
	}
	if len(p.Children[1].Marks) != 1 || p.Children[1].Marks[0].Type != document.MarkBold {
		t.Errorf("text[1].Marks = %v; want [MarkBold]", p.Children[1].Marks)
	}
}

func TestProvider_ContentRenderer_ComplexADF(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	provider, _ := newTestProvider(t, handler)
	renderer := provider.ContentRenderer()

	// Test with raw JSON bytes (as would come from Jira API).
	adfJSON := []byte(`{
		"version": 1,
		"type": "doc",
		"content": [
			{
				"type": "heading",
				"attrs": {"level": 2},
				"content": [{"type": "text", "text": "Acceptance Criteria"}]
			},
			{
				"type": "bulletList",
				"content": [
					{
						"type": "listItem",
						"content": [{
							"type": "paragraph",
							"content": [{"type": "text", "text": "First item"}]
						}]
					}
				]
			},
			{
				"type": "codeBlock",
				"attrs": {"language": "sql"},
				"content": [{"type": "text", "text": "SELECT 1;"}]
			}
		]
	}`)

	node, err := renderer.ParseContent(json.RawMessage(adfJSON))
	if err != nil {
		t.Fatalf("ParseContent(JSON) error = %v", err)
	}

	if len(node.Children) != 3 {
		t.Fatalf("len(Children) = %d; want 3", len(node.Children))
	}

	// Heading.
	if node.Children[0].Type != document.NodeHeading || node.Children[0].Level != 2 {
		t.Errorf("Children[0] = {%v, level=%d}; want {NodeHeading, 2}", node.Children[0].Type, node.Children[0].Level)
	}

	// Bullet list.
	if node.Children[1].Type != document.NodeBulletList {
		t.Errorf("Children[1].Type = %v; want NodeBulletList", node.Children[1].Type)
	}

	// Code block.
	if node.Children[2].Type != document.NodeCodeBlock || node.Children[2].Language != "sql" {
		t.Errorf("Children[2] = {%v, lang=%q}; want {NodeCodeBlock, \"sql\"}", node.Children[2].Type, node.Children[2].Language)
	}

	// Verify markdown rendering works end-to-end.
	md := document.RenderMarkdown(node)
	for _, want := range []string{"## Acceptance Criteria", "First item", "```sql", "SELECT 1;"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q in:\n%s", want, md)
		}
	}
}

func TestProvider_ContentRenderer_Table(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	provider, _ := newTestProvider(t, handler)
	renderer := provider.ContentRenderer()

	adfJSON := json.RawMessage(`{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "table",
			"content": [{
				"type": "tableRow",
				"content": [
					{"type": "tableHeader", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Name"}]}]},
					{"type": "tableCell", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Value"}]}]}
				]
			}]
		}]
	}`)

	node, err := renderer.ParseContent(adfJSON)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}
	if node.Children[0].Type != document.NodeTable {
		t.Errorf("Children[0].Type = %v; want NodeTable", node.Children[0].Type)
	}
	row := node.Children[0].Children[0]
	if len(row.Children) != 2 {
		t.Fatalf("row children = %d; want 2", len(row.Children))
	}
	if row.Children[0].Type != document.NodeTableHeader {
		t.Errorf("cell[0].Type = %v; want NodeTableHeader", row.Children[0].Type)
	}

	// Roundtrip.
	rendered, _ := renderer.RenderContent(node)
	node2, err := renderer.ParseContent(rendered)
	if err != nil {
		t.Fatalf("roundtrip ParseContent() error = %v", err)
	}
	if node2.Children[0].Type != document.NodeTable {
		t.Errorf("roundtrip type = %v; want NodeTable", node2.Children[0].Type)
	}
}

func TestProvider_ContentRenderer_UnknownNodes(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	provider, _ := newTestProvider(t, handler)
	renderer := provider.ContentRenderer()

	// "panel" is an unknown node type — children should be preserved.
	adfJSON := json.RawMessage(`{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "panel",
			"content": [{
				"type": "paragraph",
				"content": [{"type": "text", "text": "inside panel"}]
			}]
		}]
	}`)

	node, err := renderer.ParseContent(adfJSON)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}
	if len(node.Children) == 0 {
		t.Fatal("doc has no children; want wrapper for unknown node")
	}
	// Unknown nodes are converted to paragraphs preserving child text.
	p := node.Children[0]
	if p.Type != document.NodeParagraph {
		t.Errorf("Children[0].Type = %v; want NodeParagraph (wrapper for unknown)", p.Type)
	}
	if len(p.Children) == 0 {
		t.Fatal("wrapped paragraph has no children")
	}
	// The inner paragraph's children should contain the text.
	found := false
	var walk func(n *document.Node)
	walk = func(n *document.Node) {
		if n.Text == "inside panel" {
			found = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(p)
	if !found {
		t.Error("text \"inside panel\" not found in parsed unknown node children")
	}
}

func TestProvider_ContentRenderer_EmptyDoc(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	provider, _ := newTestProvider(t, handler)
	renderer := provider.ContentRenderer()

	adfJSON := json.RawMessage(`{"version": 1, "type": "doc", "content": []}`)
	node, err := renderer.ParseContent(adfJSON)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}
	if len(node.Children) != 0 {
		t.Errorf("Children = %d; want 0", len(node.Children))
	}
	if md := document.RenderMarkdown(node); strings.TrimSpace(md) != "" {
		t.Errorf("RenderMarkdown(empty) = %q; want empty", md)
	}
}

func TestProvider_ContentRenderer_NilNode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	provider, _ := newTestProvider(t, handler)
	renderer := provider.ContentRenderer()

	rendered, err := renderer.RenderContent(nil)
	if err != nil {
		t.Fatalf("RenderContent(nil) error = %v", err)
	}
	adf, ok := rendered.(map[string]any)
	if !ok {
		t.Fatalf("type = %T; want map[string]any", rendered)
	}
	if adf["type"] != "doc" {
		t.Errorf("type = %v; want \"doc\"", adf["type"])
	}
}

func TestProvider_ContentRenderer_AllMarks(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	provider, _ := newTestProvider(t, handler)
	renderer := provider.ContentRenderer()

	adfJSON := json.RawMessage(`{
		"version": 1,
		"type": "doc",
		"content": [{
			"type": "paragraph",
			"content": [
				{"type": "text", "text": "bold", "marks": [{"type": "strong"}]},
				{"type": "text", "text": "italic", "marks": [{"type": "em"}]},
				{"type": "text", "text": "code", "marks": [{"type": "code"}]},
				{"type": "text", "text": "strike", "marks": [{"type": "strike"}]},
				{"type": "text", "text": "underline", "marks": [{"type": "underline"}]},
				{"type": "text", "text": "link", "marks": [{"type": "link", "attrs": {"href": "https://example.com"}}]}
			]
		}]
	}`)

	node, err := renderer.ParseContent(adfJSON)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}

	p := node.Children[0]
	expected := []document.MarkType{
		document.MarkBold, document.MarkItalic, document.MarkCode,
		document.MarkStrike, document.MarkUnderline, document.MarkLink,
	}
	if len(p.Children) != len(expected) {
		t.Fatalf("paragraph has %d children; want %d", len(p.Children), len(expected))
	}
	for i, want := range expected {
		if len(p.Children[i].Marks) != 1 || p.Children[i].Marks[0].Type != want {
			t.Errorf("child[%d] marks = %v; want [%v]", i, p.Children[i].Marks, want)
		}
	}

	// Roundtrip through render+parse.
	rendered, _ := renderer.RenderContent(node)
	node2, err := renderer.ParseContent(rendered)
	if err != nil {
		t.Fatalf("roundtrip error = %v", err)
	}
	p2 := node2.Children[0]
	for i, want := range expected {
		if len(p2.Children[i].Marks) != 1 || p2.Children[i].Marks[0].Type != want {
			t.Errorf("roundtrip child[%d] marks = %v; want [%v]", i, p2.Children[i].Marks, want)
		}
	}
}

func TestHydrateWorkspace(t *testing.T) {
	ws := &core.Workspace{
		Slug: "eng",
		Name: "Engineering",
		Filters: map[string]string{
			"active": `status IN ("To Do", "In Progress")`,
		},
		ProviderConfig: map[string]any{
			"server":      "https://test.atlassian.net",
			"project_key": "ENG",
			"jql":         `project = "{project_key}"`,
			"board_id":    float64(42),
			"team_uuid":   "uuid-123",
			"custom_fields": map[string]any{
				"team": float64(15000),
			},
		},
	}

	cfg, err := jira.HydrateWorkspace(ws)
	if err != nil {
		t.Fatalf("HydrateWorkspace() error = %v", err)
	}

	if cfg.ProjectKey != "ENG" {
		t.Errorf("ProjectKey = %q; want \"ENG\"", cfg.ProjectKey)
	}
	if cfg.BoardID != 42 {
		t.Errorf("BoardID = %d; want 42", cfg.BoardID)
	}
	if cfg.TeamUUID != "uuid-123" {
		t.Errorf("TeamUUID = %q; want \"uuid-123\"", cfg.TeamUUID)
	}

	// ProviderConfig should now be typed.
	if _, ok := ws.ProviderConfig.(*jira.Config); !ok {
		t.Errorf("ws.ProviderConfig type = %T; want *jira.Config", ws.ProviderConfig)
	}
}

func TestHydrateWorkspace_InvalidJQL(t *testing.T) {
	ws := &core.Workspace{
		Slug: "bad",
		ProviderConfig: map[string]any{
			"server":      "https://test.atlassian.net",
			"project_key": "BAD",
			"jql":         `project = "{nonexistent_field}"`,
			"board_id":    float64(1),
		},
	}

	_, err := jira.HydrateWorkspace(ws)
	if err == nil {
		t.Fatal("HydrateWorkspace() error = nil; want error for undefined JQL variable")
	}
	if !strings.Contains(err.Error(), "nonexistent_field") {
		t.Errorf("error = %v; want containing \"nonexistent_field\"", err)
	}
}

func TestHydrateWorkspace_EmptyJQL(t *testing.T) {
	ws := &core.Workspace{
		Slug: "empty",
		ProviderConfig: map[string]any{
			"server":      "https://test.atlassian.net",
			"project_key": "X",
			"jql":         "",
			"board_id":    float64(1),
		},
	}

	_, err := jira.HydrateWorkspace(ws)
	if err == nil {
		t.Fatal("HydrateWorkspace() error = nil; want error for empty JQL")
	}
}
