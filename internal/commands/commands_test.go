package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
)

// keys returns the map keys for error messages.
func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// --- Assign ---

func TestAssign_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/myself"):
			_ = json.NewEncoder(w).Encode(client.User{AccountID: "acc-1", DisplayName: "Me"})
		case strings.Contains(r.URL.Path, "/assignee"):
			if r.Method != http.MethodPut {
				t.Errorf("method = %s; want PUT", r.Method)
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
		t.Errorf("HasNotification(\"Jira Updated\") = false; want true")
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
		t.Errorf("HasNotification(\"Jira Error\") = false; want true")
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
		t.Errorf("ClipboardContents = %q; want containing \"git checkout -b foo-42-fix-the-login-page\"", ui.ClipboardContents)
	}
}

func TestBranch_NotInCache(t *testing.T) {
	ui := &MockUI{}
	app := testApp(ui)
	app.CacheDir = t.TempDir()

	err := Branch(app, "MISSING-1", "eng")
	if err == nil {
		t.Errorf("Branch(\"MISSING-1\") = nil; want error for missing issue")
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
		t.Errorf("HasNotification(\"Jira Comment\") = false; want true")
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
		t.Errorf("HasNotification(\"ENG-5\") = false; want true")
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
		t.Errorf("Export output missing \"metadata\" key; got keys: %v", keys(output))
	}
	if _, ok := output["issues"]; !ok {
		t.Errorf("Export output missing \"issues\" key; got keys: %v", keys(output))
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

	t.Run("with parent", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeWithParent, registry)
		if len(keys) != 2 {
			t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
		}
		if !keys["C-1"] || !keys["P-1"] {
			t.Errorf("keys = %v, expected C-1 and P-1", keys)
		}
	})

	t.Run("entire board", func(t *testing.T) {
		keys := CollectExtractKeys("C-1", ScopeEntireBoard, registry)
		if len(keys) != len(registry) {
			t.Fatalf("expected %d keys (all registry), got %d", len(registry), len(keys))
		}
	})

	t.Run("missing target returns single key", func(t *testing.T) {
		keys := CollectExtractKeys("MISSING-99", ScopeFullFamily, registry)
		if len(keys) != 1 || !keys["MISSING-99"] {
			t.Errorf("keys = %v, expected only MISSING-99", keys)
		}
	})
}

func TestScopeOptions(t *testing.T) {
	t.Run("with parent", func(t *testing.T) {
		opts := ScopeOptions(true)
		if len(opts) != 5 {
			t.Fatalf("expected 5 scope options with parent, got %d", len(opts))
		}
		if opts[0] != ScopeSelectedOnly {
			t.Errorf("first option should be %q, got %q", ScopeSelectedOnly, opts[0])
		}
		if opts[4] != ScopeEntireBoard {
			t.Errorf("last option should be %q, got %q", ScopeEntireBoard, opts[4])
		}
	})

	t.Run("without parent", func(t *testing.T) {
		opts := ScopeOptions(false)
		if len(opts) != 3 {
			t.Fatalf("expected 3 scope options without parent, got %d", len(opts))
		}
		// Should not contain parent-related options.
		for _, o := range opts {
			if o == ScopeWithParent || o == ScopeFullFamily {
				t.Errorf("should not contain %q when hasParent=false", o)
			}
		}
	})
}

func TestBuildExtractXML(t *testing.T) {
	parent := &jira.IssueView{Key: "P-1", Summary: "Parent Epic", Type: "Epic", Status: "Open", Children: make(map[string]*jira.IssueView)}
	child := &jira.IssueView{Key: "C-1", Summary: "Child Story", Type: "Story", Status: "To Do", ParentKey: "P-1", Children: make(map[string]*jira.IssueView)}

	registry := map[string]*jira.IssueView{"P-1": parent, "C-1": child}
	jira.LinkChildren(registry)

	board := testConfig.Boards["eng"]

	t.Run("single issue uses markdown format", func(t *testing.T) {
		keys := map[string]bool{"C-1": true}
		xml := BuildExtractXML("Describe this issue", keys, registry, board)
		if !strings.Contains(xml, "<instruction>") {
			t.Fatal("missing <instruction> tag")
		}
		if !strings.Contains(xml, "Describe this issue") {
			t.Fatal("missing prompt text")
		}
		if !strings.Contains(xml, "Markdown") {
			t.Fatal("single issue should use Markdown output format")
		}
		if !strings.Contains(xml, `key="C-1"`) {
			t.Fatal("missing issue key in XML")
		}
	})

	t.Run("multiple issues uses schema format", func(t *testing.T) {
		keys := map[string]bool{"P-1": true, "C-1": true}
		xml := BuildExtractXML("Refine these issues", keys, registry, board)
		if !strings.Contains(xml, "json_schema") {
			t.Fatal("multiple issues should include JSON schema")
		}
		if !strings.Contains(xml, `key="P-1"`) || !strings.Contains(xml, `key="C-1"`) {
			t.Fatal("missing issue keys in XML")
		}
	})

	t.Run("parent key attribute included", func(t *testing.T) {
		keys := map[string]bool{"C-1": true}
		xml := BuildExtractXML("test", keys, registry, board)
		if !strings.Contains(xml, `parent="P-1"`) {
			t.Fatal("child issue should include parent attribute")
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
	if got := first("", "", "c"); got != "c" {
		t.Errorf("first(\"\", \"\", \"c\") = %q; want \"c\"", got)
	}
	if got := first("a", "b"); got != "a" {
		t.Errorf("first(\"a\", \"b\") = %q; want \"a\"", got)
	}
	if got := first(""); got != "" {
		t.Errorf("first(\"\") = %q; want \"\"", got)
	}
}

// --- CancelledError ---

func TestCancelledError(t *testing.T) {
	err := &CancelledError{Operation: "test"}
	if err.Error() != "test cancelled" {
		t.Errorf("CancelledError.Error() = %q; want \"test cancelled\"", err.Error())
	}
	if !IsCancelled(err) {
		t.Errorf("IsCancelled(&CancelledError{}) = false; want true")
	}
	if IsCancelled(nil) {
		t.Errorf("IsCancelled(nil) = true; want false")
	}
	if IsCancelled(os.ErrNotExist) {
		t.Errorf("IsCancelled(os.ErrNotExist) = true; want false")
	}
}

// --- ParseComment ---

func TestParseComment(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantParas int // expected number of top-level AST children (-1 to skip check)
	}{
		{"simple text", "Hello world", false, 1},
		{"with formatting", "**bold** and *italic*", false, 1},
		{"empty string", "", false, 0},
		{"multiline", "Line 1\n\nLine 2", false, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adf, ast, err := ParseComment(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseComment(%q) = nil error; want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseComment(%q) error = %v; want nil", tt.input, err)
			}
			if adf == nil {
				t.Errorf("ParseComment(%q) adf = nil; want non-nil", tt.input)
			}
			if adf["type"] != "doc" {
				t.Errorf("ParseComment(%q) adf[\"type\"] = %v; want \"doc\"", tt.input, adf["type"])
			}
			if ast == nil {
				t.Fatalf("ParseComment(%q) ast = nil; want non-nil", tt.input)
			}
			if ast.Type != document.NodeDoc {
				t.Errorf("ParseComment(%q) ast.Type = %v; want NodeDoc", tt.input, ast.Type)
			}
			if tt.wantParas >= 0 && len(ast.Children) != tt.wantParas {
				t.Errorf("ParseComment(%q) ast children = %d; want %d", tt.input, len(ast.Children), tt.wantParas)
			}
		})
	}
}

// --- FilterAndSortTransitions ---

func TestFilterAndSortTransitions(t *testing.T) {
	tests := []struct {
		name        string
		transitions []client.Transition
		allowed     []string
		wantNames   []string
	}{
		{
			"reorders to match config",
			[]client.Transition{{ID: "3", Name: "Done"}, {ID: "2", Name: "In Progress"}, {ID: "1", Name: "To Do"}},
			[]string{"To Do", "In Progress", "Done"},
			[]string{"To Do", "In Progress", "Done"},
		},
		{
			"filters out unlisted",
			[]client.Transition{{ID: "1", Name: "To Do"}, {ID: "2", Name: "In Progress"}, {ID: "3", Name: "Done"}, {ID: "4", Name: "Blocked"}},
			[]string{"To Do", "Done"},
			[]string{"To Do", "Done"},
		},
		{
			"empty allowed passes all",
			[]client.Transition{{ID: "1", Name: "A"}, {ID: "2", Name: "B"}},
			[]string{},
			[]string{"A", "B"},
		},
		{
			"case insensitive",
			[]client.Transition{{ID: "1", Name: "in progress"}},
			[]string{"In Progress"},
			[]string{"in progress"},
		},
		{
			"no matches returns empty",
			[]client.Transition{{ID: "1", Name: "Open"}},
			[]string{"Closed"},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterAndSortTransitions(tt.transitions, tt.allowed)
			gotNames := make([]string, len(got))
			for i, tr := range got {
				gotNames[i] = tr.Name
			}
			if !reflect.DeepEqual(gotNames, tt.wantNames) {
				t.Errorf("FilterAndSortTransitions() = %v; want %v", gotNames, tt.wantNames)
			}
		})
	}
}

// --- GenerateBranchCmd ---

func TestGenerateBranchCmd(t *testing.T) {
	tests := []struct {
		name     string
		issueKey string
		summary  string
		want     string
	}{
		{"basic", "ENG-42", "Fix the Login Page", "git checkout -b eng-42-fix-the-login-page"},
		{"special chars", "FOO-1", "Add OAuth 2.0 (Google)", "git checkout -b foo-1-add-oauth-2-0-google"},
		{"trailing hyphens trimmed", "BAR-5", "---urgent---", "git checkout -b bar-5-urgent"},
		{"empty summary", "X-1", "", "git checkout -b x-1-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBranchCmd(tt.issueKey, tt.summary)
			if got != tt.want {
				t.Errorf("GenerateBranchCmd(%q, %q) = %q; want %q", tt.issueKey, tt.summary, got, tt.want)
			}
		})
	}
}

// --- PostUpsertNotifications ---

func TestPostUpsertNotifications(t *testing.T) {
	// sprintAndTransitionServer returns an httptest server that handles:
	//   GET /rest/agile/1.0/board/{id}/sprint → active sprint
	//   POST /rest/agile/1.0/sprint/{id}/issue → accept sprint move
	//   GET /rest/api/2/issue/{key}/transitions → transitions list
	//   POST /rest/api/2/issue/{key}/transitions → do transition
	sprintAndTransitionServer := func(t *testing.T, sprintErr bool, transitions []client.Transition) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.Contains(r.URL.Path, "/sprint") && strings.Contains(r.URL.Path, "/issue"):
				// POST to add issue to sprint
				w.WriteHeader(204)
			case strings.Contains(r.URL.Path, "/sprint"):
				// GET active sprint
				if sprintErr {
					w.WriteHeader(500)
					_, _ = w.Write([]byte("internal error"))
					return
				}
				resp := struct {
					Values []client.Sprint `json:"values"`
				}{
					Values: []client.Sprint{{ID: 100, Name: "Sprint 1", State: "active"}},
				}
				_ = json.NewEncoder(w).Encode(resp)
			case strings.Contains(r.URL.Path, "/transitions"):
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(client.TransitionsResponse{Transitions: transitions})
				} else {
					w.WriteHeader(204)
				}
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(404)
			}
		}))
	}

	board := testConfig.Boards["eng"]

	tests := []struct {
		name       string
		fm         map[string]string
		origStatus string
		sprintErr  bool
		transitions []client.Transition
		wantContains   []string
		wantNotContain []string
		wantLen    int // expected number of notes (-1 to skip)
	}{
		{
			name:       "sprint true adds to sprint",
			fm:         map[string]string{"sprint": "true", "status": "To Do"},
			origStatus: "To Do",
			transitions: []client.Transition{},
			wantContains:   []string{"Added"},
			wantNotContain: []string{"→"},
			wantLen:    1,
		},
		{
			name:        "status change transitions",
			fm:          map[string]string{"status": "Done"},
			origStatus:  "To Do",
			transitions: []client.Transition{{ID: "30", Name: "Done"}},
			wantContains:   []string{"→ Done"},
			wantLen:     1,
		},
		{
			name:       "same status skips transition",
			fm:         map[string]string{"status": "To Do"},
			origStatus: "To Do",
			wantLen:    0,
		},
		{
			name:       "empty status skips transition",
			fm:         map[string]string{"status": ""},
			origStatus: "Open",
			wantLen:    0,
		},
		{
			name:         "sprint error reported",
			fm:           map[string]string{"sprint": "true"},
			origStatus:   "",
			sprintErr:    true,
			wantContains: []string{"Sprint Error"},
			wantLen:      1,
		},
		{
			name:         "transition error when no matching transition",
			fm:           map[string]string{"status": "Nope"},
			origStatus:   "Open",
			transitions:  []client.Transition{{ID: "1", Name: "Done"}},
			wantContains: []string{"Could not transition"},
			wantLen:      1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := sprintAndTransitionServer(t, tt.sprintErr, tt.transitions)
			defer srv.Close()

			ui := &MockUI{}
			app := testApp(ui)
			app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))

			notes := PostUpsertNotifications(app, board, tt.fm, "ENG-1", tt.origStatus)

			if tt.wantLen >= 0 && len(notes) != tt.wantLen {
				t.Errorf("PostUpsertNotifications() returned %d notes; want %d: %v", len(notes), tt.wantLen, notes)
			}
			joined := strings.Join(notes, " | ")
			for _, s := range tt.wantContains {
				if !strings.Contains(joined, s) {
					t.Errorf("PostUpsertNotifications() notes = %v; want containing %q", notes, s)
				}
			}
			for _, s := range tt.wantNotContain {
				if strings.Contains(joined, s) {
					t.Errorf("PostUpsertNotifications() notes = %v; want NOT containing %q", notes, s)
				}
			}
		})
	}
}

