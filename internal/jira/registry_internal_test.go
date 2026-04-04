package jira

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Jira Cloud datetime with positive offset",
			input: "2026-03-30T19:34:19.425+0100",
			want:  "2026-03-30T19:34:19+01:00",
		},
		{
			name:  "Jira Cloud datetime with UTC offset",
			input: "2026-01-15T10:30:00.000+0000",
			want:  "2026-01-15T10:30:00Z",
		},
		{
			name:  "Jira Cloud datetime with negative offset",
			input: "2025-12-01T08:15:42.123-0500",
			want:  "2025-12-01T08:15:42-05:00",
		},
		{
			name:  "date-only fallback",
			input: "2026-03-30",
			want:  "2026-03-30",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "short string",
			input: "2026",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDate(tt.input)
			if got != tt.want {
				t.Errorf("formatDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
			// All non-empty results must be valid RFC3339 or ISO 8601 date.
			if got != "" {
				if _, err := time.Parse(time.RFC3339, got); err != nil {
					if _, err := time.Parse("2006-01-02", got); err != nil {
						t.Errorf("formatDate(%q) = %q is not valid RFC3339 or ISO date", tt.input, got)
					}
				}
			}
		})
	}
}

func TestCustomSprint(t *testing.T) {
	tests := []struct {
		name    string
		customs map[string][]byte
		fieldID string
		want    string
	}{
		{
			name: "active sprint returned",
			customs: map[string][]byte{
				"customfield_10020": []byte(`[{"id":1,"name":"Sprint 1","state":"closed"},{"id":2,"name":"Sprint 2","state":"active"}]`),
			},
			fieldID: "customfield_10020",
			want:    "Sprint 2",
		},
		{
			name: "no active sprint falls back to last",
			customs: map[string][]byte{
				"customfield_10020": []byte(`[{"id":1,"name":"Sprint 1","state":"closed"},{"id":2,"name":"Sprint 2","state":"future"}]`),
			},
			fieldID: "customfield_10020",
			want:    "Sprint 2",
		},
		{
			name: "single sprint",
			customs: map[string][]byte{
				"customfield_10020": []byte(`[{"id":1,"name":"Sprint 1","state":"active"}]`),
			},
			fieldID: "customfield_10020",
			want:    "Sprint 1",
		},
		{
			name:    "missing field",
			customs: map[string][]byte{},
			fieldID: "customfield_10020",
			want:    "",
		},
		{
			name: "null value",
			customs: map[string][]byte{
				"customfield_10020": []byte(`null`),
			},
			fieldID: "customfield_10020",
			want:    "",
		},
		{
			name: "empty array",
			customs: map[string][]byte{
				"customfield_10020": []byte(`[]`),
			},
			fieldID: "customfield_10020",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &issueFields{
				Customs: make(map[string]json.RawMessage),
			}
			for k, v := range tt.customs {
				f.Customs[k] = v
			}
			got := f.CustomSprint(tt.fieldID)
			if got != tt.want {
				t.Errorf("CustomSprint(%q) = %q, want %q", tt.fieldID, got, tt.want)
			}
		})
	}
}
