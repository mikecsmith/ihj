package jira

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
)

func TestFilterTransitions_NoFilter(t *testing.T) {
	transitions := []client.Transition{
		{ID: "1", Name: "To Do"},
		{ID: "2", Name: "In Progress"},
		{ID: "3", Name: "Done"},
	}
	filtered := FilterTransitions(transitions, nil)
	if len(filtered) != 3 {
		t.Errorf("FilterTransitions(nil) len = %d; want 3", len(filtered))
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
		t.Fatalf("FilterTransitions() len = %d; want 2", len(filtered))
	}
	if filtered[0].Name != "To Do" || filtered[1].Name != "Done" {
		t.Errorf("FilterTransitions() = [%q, %q]; want [\"To Do\", \"Done\"]", filtered[0].Name, filtered[1].Name)
	}
}

func TestFilterTransitions_CaseInsensitive(t *testing.T) {
	transitions := []client.Transition{{ID: "1", Name: "In Progress"}}
	filtered := FilterTransitions(transitions, []string{"in progress"})
	if len(filtered) != 1 {
		t.Errorf("FilterTransitions(case-insensitive) len = %d; want 1", len(filtered))
	}
}

func TestFindTransitionID(t *testing.T) {
	transitions := []client.Transition{
		{ID: "10", Name: "Start", To: client.Status{Name: "In Progress"}},
		{ID: "20", Name: "Finish", To: client.Status{Name: "Done"}},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"by name", "Start", "10"},
		{"by to.name", "Done", "20"},
		{"missing", "Missing", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FindTransitionID(transitions, tt.input); got != tt.want {
				t.Errorf("FindTransitionID(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
