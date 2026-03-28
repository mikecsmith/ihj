package commands_test

import (
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/testutil"
)

func TestTransition_Success(t *testing.T) {
	ui := &testutil.MockUI{SelectReturn: 2} // Select "In Progress" (index 2 in Backlog/To Do/In Progress/In Review/Done)
	provider := &testutil.MockProvider{
		Caps: core.Capabilities{HasTransitions: true},
	}
	s := testutil.NewTestSession(ui)
	s.Provider = provider

	err := commands.Transition(s, "", "ENG-5")
	if err != nil {
		t.Fatal(err)
	}

	if !ui.HasNotification("ENG-5") {
		t.Errorf("hasNotification(\"ENG-5\") = false; want true")
	}

	if len(provider.UpdateCalls) != 1 {
		t.Fatalf("UpdateCalls = %d; want 1", len(provider.UpdateCalls))
	}
	call := provider.UpdateCalls[0]
	if call.ID != "ENG-5" {
		t.Errorf("Update ID = %q; want \"ENG-5\"", call.ID)
	}
	if call.Changes.Status == nil || *call.Changes.Status != "In Progress" {
		t.Errorf("Update Status = %v; want \"In Progress\"", call.Changes.Status)
	}
}

func TestTransition_Cancel(t *testing.T) {
	ui := &testutil.MockUI{SelectReturn: -1}
	provider := &testutil.MockProvider{
		Caps: core.Capabilities{HasTransitions: true},
	}
	s := testutil.NewTestSession(ui)
	s.Provider = provider

	err := commands.Transition(s, "", "ENG-1")
	if !commands.IsCancelled(err) {
		t.Errorf("expected CancelledError, got %v", err)
	}
}

func TestTransition_NoCapability(t *testing.T) {
	ui := &testutil.MockUI{}
	provider := &testutil.MockProvider{
		Caps: core.Capabilities{HasTransitions: false},
	}
	s := testutil.NewTestSession(ui)
	s.Provider = provider

	err := commands.Transition(s, "", "ENG-1")
	if err == nil {
		t.Fatal("expected error for provider without transitions")
	}
}
