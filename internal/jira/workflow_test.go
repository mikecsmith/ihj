package jira

import (
	"testing"

)

func TestFindTransitionID(t *testing.T) {
	transitions := []Transition{
		{ID: "10", Name: "Start", To: Status{Name: "In Progress"}},
		{ID: "20", Name: "Finish", To: Status{Name: "Done"}},
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
