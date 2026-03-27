package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/jira"
)

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

func TestPostUpsertNotifications(t *testing.T) {
	// sprintAndTransitionServer returns an httptest server that handles:
	//   GET /rest/agile/1.0/board/{id}/sprint → active sprint
	//   POST /rest/agile/1.0/sprint/{id}/issue → accept sprint move
	//   GET /rest/api/2/issue/{key}/transitions → transitions list
	//   POST /rest/api/2/issue/{key}/transitions → do transition
	sprintAndTransitionServer := func(t *testing.T, sprintErr bool, transitions []jira.Transition) *httptest.Server {
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
					Values []jira.Sprint `json:"values"`
				}{
					Values: []jira.Sprint{{ID: 100, Name: "Sprint 1", State: "active"}},
				}
				_ = json.NewEncoder(w).Encode(resp)
			case strings.Contains(r.URL.Path, "/transitions"):
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(jira.TransitionsResponse{Transitions: transitions})
				} else {
					w.WriteHeader(204)
				}
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(404)
			}
		}))
	}

	board := testConfig.Workspaces["eng"]

	tests := []struct {
		name           string
		fm             map[string]string
		origStatus     string
		sprintErr      bool
		transitions    []jira.Transition
		wantContains   []string
		wantNotContain []string
		wantLen        int // expected number of notes (-1 to skip)
	}{
		{
			name:           "sprint true adds to sprint",
			fm:             map[string]string{"sprint": "true", "status": "To Do"},
			origStatus:     "To Do",
			transitions:    []jira.Transition{},
			wantContains:   []string{"Added"},
			wantNotContain: []string{"→"},
			wantLen:        1,
		},
		{
			name:         "status change transitions",
			fm:           map[string]string{"status": "Done"},
			origStatus:   "To Do",
			transitions:  []jira.Transition{{ID: "30", Name: "Done"}},
			wantContains: []string{"→ Done"},
			wantLen:      1,
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
			transitions:  []jira.Transition{{ID: "1", Name: "Done"}},
			wantContains: []string{"Could not transition"},
			wantLen:      1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := sprintAndTransitionServer(t, tt.sprintErr, tt.transitions)
			defer srv.Close()

			ui := &MockUI{}
			app := NewTestApp(ui)
			app.Client = jira.New(srv.URL, "token", jira.WithMaxRetries(0))

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
