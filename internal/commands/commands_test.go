package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/jira"
)

// --- Assign ---

func TestAssign_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/myself"):
			_ = json.NewEncoder(w).Encode(client.User{AccountID: "acc-1", DisplayName: "Me"})
		case strings.Contains(r.URL.Path, "/assignee"):
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	ui := &MockUI{}
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))
	app.CacheDir = t.TempDir()

	err := Assign(app, "FOO-1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if !ui.HasNotification("Jira Updated") {
		t.Error("expected success notification")
	}
}

func TestAssign_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/myself") {
			_ = json.NewEncoder(w).Encode(client.User{AccountID: "acc-1"})
			return
		}
		w.WriteHeader(403)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	ui := &MockUI{}
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))
	app.CacheDir = t.TempDir()

	err := Assign(app, "FOO-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !ui.HasNotification("Jira Error") {
		t.Error("expected error notification")
	}
}

// --- Branch ---

func TestBranch_FromCache(t *testing.T) {
	dir := t.TempDir()
	issues := []client.Issue{
		{Key: "FOO-42", Fields: client.IssueFields{
			Summary:   "Fix the Login Page",
			IssueType: client.IssueType{ID: "1", Name: "Bug"},
			Status:    client.Status{Name: "Open"},
			Priority:  client.Priority{Name: "High"},
			Created:   "2024-01-01T00:00:00.000+0000",
			Updated:   "2024-01-01T00:00:00.000+0000",
		}},
	}
	data, _ := json.Marshal(issues)
	_ = os.WriteFile(filepath.Join(dir, "eng_active.json"), data, 0o644)

	ui := &MockUI{}
	app := testApp(ui)
	app.CacheDir = dir

	err := Branch(app, "FOO-42", "eng")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(ui.ClipboardContents, "git checkout -b foo-42-fix-the-login-page") {
		t.Errorf("clipboard = %q", ui.ClipboardContents)
	}
}

func TestBranch_NotInCache(t *testing.T) {
	ui := &MockUI{}
	app := testApp(ui)
	app.CacheDir = t.TempDir()

	err := Branch(app, "MISSING-1", "eng")
	if err == nil {
		t.Error("expected error for missing issue")
	}
}

// --- Comment ---

func TestComment_EmptyAbort(t *testing.T) {
	ui := &MockUI{EditTextReturn: "   "}
	app := testApp(ui)

	err := Comment(app, "FOO-1")
	if !IsCancelled(err) {
		t.Errorf("expected CancelledError, got %v", err)
	}
}

func TestComment_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/comment") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	ui := &MockUI{EditTextReturn: "This is my comment."}
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))

	err := Comment(app, "FOO-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ui.HasNotification("Jira Comment") {
		t.Error("expected success notification")
	}
}

// --- Transition ---

func TestTransition_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(client.TransitionsResponse{
				Transitions: []client.Transition{
					{ID: "10", Name: "To Do"},
					{ID: "20", Name: "In Progress"},
					{ID: "30", Name: "Done"},
				},
			})
			return
		}
		// POST transition
		w.WriteHeader(204)
	}))
	defer srv.Close()

	ui := &MockUI{SelectReturn: 1} // Select "In Progress"
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))

	err := Transition(app, "ENG-5")
	if err != nil {
		t.Fatal(err)
	}
	if !ui.HasNotification("ENG-5") {
		t.Error("expected transition notification")
	}
}

func TestTransition_Cancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(client.TransitionsResponse{
			Transitions: []client.Transition{{ID: "1", Name: "Done"}},
		})
	}))
	defer srv.Close()

	ui := &MockUI{SelectReturn: -1}
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))

	err := Transition(app, "ENG-1")
	if !IsCancelled(err) {
		t.Errorf("expected CancelledError, got %v", err)
	}
}

// --- Export ---

func TestExport_WritesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(client.SearchResponse{
			Issues: []client.Issue{
				{Key: "ENG-1", Fields: client.IssueFields{
					Summary:   "Test",
					IssueType: client.IssueType{ID: "10", Name: "Story"},
					Status:    client.Status{Name: "Open"},
					Priority:  client.Priority{Name: "Medium"},
					Created:   "2024-01-01T00:00:00.000+0000",
					Updated:   "2024-01-01T00:00:00.000+0000",
				}},
			},
			IsLast: true,
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	ui := &MockUI{}
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))
	app.CacheDir = t.TempDir()
	app.Out = &buf

	err := Export(app, "eng", "active")
	if err != nil {
		t.Fatal(err)
	}

	var output map[string]any
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := output["metadata"]; !ok {
		t.Error("missing metadata in output")
	}
	if _, ok := output["issues"]; !ok {
		t.Error("missing issues in output")
	}
}

// --- Extract ---

func TestCollectExtractKeys(t *testing.T) {
	parent := &jira.IssueView{Key: "P-1", Summary: "Parent", Type: "Epic", Status: "Open", Children: make(map[string]*jira.IssueView)}
	child1 := &jira.IssueView{Key: "C-1", Summary: "Child 1", Type: "Story", Status: "Open", ParentKey: "P-1", Children: make(map[string]*jira.IssueView)}
	child2 := &jira.IssueView{Key: "C-2", Summary: "Child 2", Type: "Story", Status: "Open", ParentKey: "P-1", Children: make(map[string]*jira.IssueView)}
	sibling := &jira.IssueView{Key: "S-1", Summary: "Sibling", Type: "Story", Status: "Open", ParentKey: "P-1", Children: make(map[string]*jira.IssueView)}

	registry := map[string]*jira.IssueView{
		"P-1": parent, "C-1": child1, "C-2": child2, "S-1": sibling,
	}
	jira.LinkChildren(registry)

	t.Run("target only", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeSelectedOnly, registry)
		if len(keys) != 1 || !keys["C-1"] {
			t.Errorf("keys = %v", keys)
		}
	})

	t.Run("target + children", func(t *testing.T) {
		keys := CollectExtractKeys("P-1", ScopeWithChildren, registry)
		if len(keys) != 4 { // P-1, C-1, C-2, S-1 are all children of P-1
			t.Errorf("keys = %v, want 4", keys)
		}
	})

	t.Run("full family", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeFullFamily, registry)
		// Should include C-1, P-1 (parent), C-2 and S-1 (siblings sharing parent P-1)
		if !keys["C-1"] || !keys["P-1"] || !keys["C-2"] || !keys["S-1"] {
			t.Errorf("keys = %v, missing expected entries", keys)
		}
	})
}

// --- Upsert helpers ---

func TestValidateFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		fm   map[string]string
		want string
	}{
		{"valid", map[string]string{"summary": "test", "type": "Story"}, ""},
		{"missing summary", map[string]string{"type": "Story"}, "Summary is required."},
		{"subtask no parent", map[string]string{"summary": "x", "type": "Sub-task"}, "Sub-tasks require a parent issue key."},
		{"subtask with parent", map[string]string{"summary": "x", "type": "Sub-task", "parent": "FOO-1"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFrontmatter(tt.fm)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateCursor_EmptySummary(t *testing.T) {
	line, pat := CalculateCursor("---\ntype: Story\nsummary: \"\"\n---\n", "")
	if pat != "^summary:" {
		t.Errorf("pattern = %q, want '^summary:'", pat)
	}
	_ = line
}

func TestCalculateCursor_WithSummary(t *testing.T) {
	doc := "---\ntype: Story\nsummary: \"test\"\n---\n\nBody here"
	line, pat := CalculateCursor(doc, "test")
	if pat != "" {
		t.Errorf("pattern = %q, want empty (cursor by line)", pat)
	}
	if line != 5 {
		t.Errorf("line = %d, want 5 (after closing ---)", line)
	}
}

func TestFirst(t *testing.T) {
	if first("", "", "c") != "c" {
		t.Error("should skip empty strings")
	}
	if first("a", "b") != "a" {
		t.Error("should return first non-empty")
	}
	if first("") != "" {
		t.Error("all empty should return empty")
	}
}

// --- CancelledError ---

func TestCancelledError(t *testing.T) {
	err := &CancelledError{Operation: "test"}
	if err.Error() != "test cancelled" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !IsCancelled(err) {
		t.Error("IsCancelled should return true")
	}
	if IsCancelled(nil) {
		t.Error("nil should not be cancelled")
	}
	if IsCancelled(os.ErrNotExist) {
		t.Error("random error should not be cancelled")
	}
}
