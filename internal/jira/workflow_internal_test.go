package jira

import (
	"context"
	"fmt"
	"testing"
)

// stubAPI embeds a zero-value Client to satisfy the API interface and
// overrides only the sprint methods needed by these tests.
type stubAPI struct {
	Client // embed to satisfy all methods; panics on unset ones are fine here

	activeSprint   *sprint
	activeErr      error
	futureSprint   *sprint
	futureErr      error
	addedSprints   []int
	addErr         error
	backloggedKeys []string
	backlogErr     error
}

func (s *stubAPI) FetchActiveSprint(_ context.Context, _ int) (*sprint, error) {
	return s.activeSprint, s.activeErr
}

func (s *stubAPI) FetchNextFutureSprint(_ context.Context, _ int) (*sprint, error) {
	return s.futureSprint, s.futureErr
}

func (s *stubAPI) AddToSprint(_ context.Context, sprintID int, _ []string) error {
	s.addedSprints = append(s.addedSprints, sprintID)
	return s.addErr
}

func (s *stubAPI) MoveToBacklog(_ context.Context, issueKeys []string) error {
	s.backloggedKeys = append(s.backloggedKeys, issueKeys...)
	return s.backlogErr
}

func TestSprintAssign_Active(t *testing.T) {
	api := &stubAPI{activeSprint: &sprint{ID: 10, Name: "Sprint 5"}}

	err := sprintAssign(context.Background(), api, 1, "FOO-1", "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(api.addedSprints) != 1 || api.addedSprints[0] != 10 {
		t.Errorf("expected AddToSprint(10), got %v", api.addedSprints)
	}
}

func TestSprintAssign_Future(t *testing.T) {
	api := &stubAPI{futureSprint: &sprint{ID: 20, Name: "Sprint 6"}}

	err := sprintAssign(context.Background(), api, 1, "FOO-1", "future")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(api.addedSprints) != 1 || api.addedSprints[0] != 20 {
		t.Errorf("expected AddToSprint(20), got %v", api.addedSprints)
	}
}

func TestSprintAssign_NoActiveSprint(t *testing.T) {
	api := &stubAPI{activeSprint: nil}

	err := sprintAssign(context.Background(), api, 42, "FOO-1", "active")
	if err == nil {
		t.Fatal("expected error for missing active sprint")
	}
	if got := err.Error(); got != "no active sprint on board 42" {
		t.Errorf("error = %q", got)
	}
}

func TestSprintAssign_NoFutureSprint(t *testing.T) {
	api := &stubAPI{futureSprint: nil}

	err := sprintAssign(context.Background(), api, 42, "FOO-1", "future")
	if err == nil {
		t.Fatal("expected error for missing future sprint")
	}
	if got := err.Error(); got != "no future sprint on board 42" {
		t.Errorf("error = %q", got)
	}
}

func TestSprintAssign_None(t *testing.T) {
	api := &stubAPI{}

	err := sprintAssign(context.Background(), api, 1, "FOO-1", "none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(api.backloggedKeys) != 1 || api.backloggedKeys[0] != "FOO-1" {
		t.Errorf("expected MoveToBacklog(FOO-1), got %v", api.backloggedKeys)
	}
	if len(api.addedSprints) != 0 {
		t.Errorf("expected no AddToSprint calls, got %v", api.addedSprints)
	}
}

func TestSprintAssign_NoneAPIError(t *testing.T) {
	api := &stubAPI{backlogErr: fmt.Errorf("board not found")}

	err := sprintAssign(context.Background(), api, 1, "FOO-1", "none")
	if err == nil {
		t.Fatal("expected error from MoveToBacklog")
	}
	if got := err.Error(); !contains(got, "board not found") {
		t.Errorf("error = %q, want containing 'board not found'", got)
	}
}

func TestSprintAssign_InvalidTarget(t *testing.T) {
	api := &stubAPI{}

	err := sprintAssign(context.Background(), api, 1, "FOO-1", "past")
	if err == nil {
		t.Fatal("expected error for invalid target")
	}
	if got := err.Error(); got != `unknown sprint target "past" (expected "active", "future", or "none")` {
		t.Errorf("error = %q", got)
	}
}

func TestSprintAssign_APIError(t *testing.T) {
	api := &stubAPI{
		activeSprint: &sprint{ID: 10, Name: "Sprint 5"},
		addErr:       fmt.Errorf("permission denied"),
	}

	err := sprintAssign(context.Background(), api, 1, "FOO-1", "active")
	if err == nil {
		t.Fatal("expected error from AddToSprint")
	}
	if got := err.Error(); !contains(got, "permission denied") {
		t.Errorf("error = %q, want containing 'permission denied'", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
