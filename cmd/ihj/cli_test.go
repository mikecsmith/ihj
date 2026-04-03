package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCollectOverrides(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "single field",
			args: []string{"--set", "priority=High"},
			want: map[string]string{"priority": "High"},
		},
		{
			name: "multiple fields",
			args: []string{"--set", "priority=High", "--set", "sprint=active"},
			want: map[string]string{"priority": "High", "sprint": "active"},
		},
		{
			name: "core fields via set",
			args: []string{"--set", "summary=Fix bug", "--set", "type=Story", "--set", "status=To Do", "--set", "parent=ENG-1"},
			want: map[string]string{"summary": "Fix bug", "type": "Story", "status": "To Do", "parent": "ENG-1"},
		},
		{
			name: "short flag -s",
			args: []string{"-s", "assignee=mike@example.com"},
			want: map[string]string{"assignee": "mike@example.com"},
		},
		{
			name: "empty value is no-op",
			args: []string{"--set", "sprint="},
			want: map[string]string{},
		},
		{
			name: "none clears field",
			args: []string{"--set", "sprint=none"},
			want: map[string]string{"sprint": "none"},
		},
		{
			name: "no flags",
			args: nil,
			want: map[string]string{},
		},
		{
			name: "value containing equals sign",
			args: []string{"--set", "summary=a=b"},
			want: map[string]string{"summary": "a=b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			addMutationFlags(cmd)
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}

			got := collectOverrides(cmd)
			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d; got %v", len(got), len(tt.want), got)
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("got[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
