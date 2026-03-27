package commands

import (
	"fmt"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestAssign_Success(t *testing.T) {
	ui := &MockUI{}
	mp := &core.MockProvider{}
	s := NewTestSession(ui)
	s.Provider = mp

	err := Assign(s, "FOO-1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if !ui.HasNotification("Assigned") {
		t.Errorf("HasNotification(\"Assigned\") = false; want true")
	}
	if len(mp.AssignCalls) != 1 || mp.AssignCalls[0] != "FOO-1" {
		t.Errorf("AssignCalls = %v; want [FOO-1]", mp.AssignCalls)
	}
}

func TestAssign_ProviderError(t *testing.T) {
	ui := &MockUI{}
	mp := &core.MockProvider{AssignErr: fmt.Errorf("forbidden")}
	s := NewTestSession(ui)
	s.Provider = mp

	err := Assign(s, "FOO-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !ui.HasNotification("Error") {
		t.Errorf("HasNotification(\"Error\") = false; want true")
	}
}
