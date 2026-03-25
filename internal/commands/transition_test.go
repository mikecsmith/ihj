package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/mikecsmith/ihj/internal/jira"
)

func TestTransition_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(jira.TransitionsResponse{
				Transitions: []jira.Transition{
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
	app := NewTestApp(ui)
	app.Client = jira.New(srv.URL, "token", jira.WithMaxRetries(0))

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
		_ = json.NewEncoder(w).Encode(jira.TransitionsResponse{
			Transitions: []jira.Transition{{ID: "1", Name: "Done"}},
		})
	}))
	defer srv.Close()

	ui := &MockUI{SelectReturn: -1}
	app := NewTestApp(ui)
	app.Client = jira.New(srv.URL, "token", jira.WithMaxRetries(0))

	err := Transition(app, "ENG-1")
	if !IsCancelled(err) {
		t.Errorf("expected CancelledError, got %v", err)
	}
}

func TestFilterAndSortTransitions(t *testing.T) {
	tests := []struct {
		name        string
		transitions []jira.Transition
		allowed     []string
		wantNames   []string
	}{
		{
			"reorders to match config",
			[]jira.Transition{{ID: "3", Name: "Done"}, {ID: "2", Name: "In Progress"}, {ID: "1", Name: "To Do"}},
			[]string{"To Do", "In Progress", "Done"},
			[]string{"To Do", "In Progress", "Done"},
		},
		{
			"filters out unlisted",
			[]jira.Transition{{ID: "1", Name: "To Do"}, {ID: "2", Name: "In Progress"}, {ID: "3", Name: "Done"}, {ID: "4", Name: "Blocked"}},
			[]string{"To Do", "Done"},
			[]string{"To Do", "Done"},
		},
		{
			"empty allowed passes all",
			[]jira.Transition{{ID: "1", Name: "A"}, {ID: "2", Name: "B"}},
			[]string{},
			[]string{"A", "B"},
		},
		{
			"case insensitive",
			[]jira.Transition{{ID: "1", Name: "in progress"}},
			[]string{"In Progress"},
			[]string{"in progress"},
		},
		{
			"no matches returns empty",
			[]jira.Transition{{ID: "1", Name: "Open"}},
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
