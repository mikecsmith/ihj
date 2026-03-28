package commands_test

import (
	"fmt"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/testutil"
)

func TestAssign_Success(t *testing.T) {
	ui := &testutil.MockUI{}
	mp := &testutil.MockProvider{}
	ws := testutil.NewTestSession(ui)
	ws.Provider = mp

	err := commands.Assign(ws, "FOO-1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if !ui.HasNotification("Assigned") {
		t.Errorf("hasNotification(\"Assigned\") = false; want true")
	}
	if len(mp.AssignCalls) != 1 || mp.AssignCalls[0] != "FOO-1" {
		t.Errorf("AssignCalls = %v; want [FOO-1]", mp.AssignCalls)
	}
}

func TestAssign_ProviderError(t *testing.T) {
	ui := &testutil.MockUI{}
	mp := &testutil.MockProvider{AssignErr: fmt.Errorf("forbidden")}
	ws := testutil.NewTestSession(ui)
	ws.Provider = mp

	err := commands.Assign(ws, "FOO-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !ui.HasNotification("Error") {
		t.Errorf("hasNotification(\"Error\") = false; want true")
	}
}
