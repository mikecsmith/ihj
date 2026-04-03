package core_test

import (
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestValidateFieldOverrides(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "priority", Label: "Priority", Type: core.FieldEnum,
			Enum: []string{"Highest", "High", "Medium", "Low", "Lowest"},
			Role: core.RoleUrgency, Primary: true},
		{Key: "assignee", Label: "Assignee", Type: core.FieldAssignee,
			Role: core.RoleOwnership, Primary: true},
		{Key: "sprint", Label: "Sprint", Type: core.FieldString,
			Role: core.RoleIteration, WriteOnly: true},
		{Key: "reporter", Label: "Reporter", Type: core.FieldEmail,
			Role: core.RoleOwnership, Derived: true},
	}

	tests := []struct {
		name      string
		overrides map[string]string
		wantErr   string
	}{
		{
			name:      "valid enum value",
			overrides: map[string]string{"priority": "High"},
		},
		{
			name:      "case insensitive enum normalises to canonical",
			overrides: map[string]string{"priority": "high"},
		},
		{
			name:      "valid string field",
			overrides: map[string]string{"sprint": "active"},
		},
		{
			name:      "core keys are always valid",
			overrides: map[string]string{"summary": "Hello", "type": "Story", "status": "To Do", "parent": "ENG-1"},
		},
		{
			name:      "multiple valid overrides",
			overrides: map[string]string{"priority": "Low", "sprint": "active"},
		},
		{
			name:      "nil overrides",
			overrides: nil,
		},
		{
			name:      "empty overrides",
			overrides: map[string]string{},
		},
		{
			name:      "unknown field",
			overrides: map[string]string{"nonexistent": "value"},
			wantErr:   `unknown field "nonexistent"`,
		},
		{
			name:      "invalid enum value",
			overrides: map[string]string{"priority": "Urgent"},
			wantErr:   `invalid value "Urgent" for field "priority"`,
		},
		{
			name:      "read-only field",
			overrides: map[string]string{"reporter": "test@example.com"},
			wantErr:   `field "reporter" is read-only`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateFieldOverrides(tt.overrides, defs)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFieldOverrides_NormalisesEnumCase(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "priority", Type: core.FieldEnum,
			Enum: []string{"Highest", "High", "Medium", "Low", "Lowest"}},
	}
	overrides := map[string]string{"priority": "high"}
	if err := core.ValidateFieldOverrides(overrides, defs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides["priority"] != "High" {
		t.Errorf("expected normalised to %q, got %q", "High", overrides["priority"])
	}
}

func TestWritableKeys(t *testing.T) {
	defs := core.FieldDefs{
		{Key: "priority", Type: core.FieldEnum, Primary: true},
		{Key: "reporter", Type: core.FieldEmail, Derived: true},
		{Key: "assignee", Type: core.FieldAssignee, Primary: true},
		{Key: "created", Type: core.FieldString, Immutable: true},
	}

	got := defs.WritableKeys()
	if !strings.Contains(got, "priority") {
		t.Errorf("expected 'priority' in %q", got)
	}
	if !strings.Contains(got, "assignee") {
		t.Errorf("expected 'assignee' in %q", got)
	}
	if strings.Contains(got, "reporter") {
		t.Errorf("derived field 'reporter' should not be in %q", got)
	}
	if strings.Contains(got, "created") {
		t.Errorf("immutable field 'created' should not be in %q", got)
	}
}
