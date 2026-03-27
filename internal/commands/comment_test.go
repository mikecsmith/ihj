package commands

import (
	"fmt"
	"testing"

	"github.com/mikecsmith/ihj/internal/core"
)

func TestComment_EmptyAbort(t *testing.T) {
	ui := &MockUI{EditTextReturn: "   "}
	s := NewTestSession(ui)
	s.Provider = &core.MockProvider{}

	err := Comment(s, "FOO-1")
	if !IsCancelled(err) {
		t.Errorf("expected CancelledError, got %v", err)
	}
}

func TestComment_Success(t *testing.T) {
	ui := &MockUI{EditTextReturn: "This is my comment."}
	mp := &core.MockProvider{}
	s := NewTestSession(ui)
	s.Provider = mp

	err := Comment(s, "FOO-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ui.HasNotification("Comment") {
		t.Errorf("HasNotification(\"Comment\") = false; want true")
	}
	if len(mp.CommentCalls) != 1 {
		t.Fatalf("CommentCalls = %d; want 1", len(mp.CommentCalls))
	}
	if mp.CommentCalls[0].ID != "FOO-1" {
		t.Errorf("CommentCalls[0].ID = %q; want FOO-1", mp.CommentCalls[0].ID)
	}
	if mp.CommentCalls[0].Body != "This is my comment." {
		t.Errorf("CommentCalls[0].Body = %q; want \"This is my comment.\"", mp.CommentCalls[0].Body)
	}
}

func TestComment_ProviderError(t *testing.T) {
	ui := &MockUI{EditTextReturn: "A comment"}
	mp := &core.MockProvider{CommentErr: fmt.Errorf("network error")}
	s := NewTestSession(ui)
	s.Provider = mp

	err := Comment(s, "FOO-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !ui.HasNotification("Error") {
		t.Errorf("HasNotification(\"Error\") = false; want true")
	}
}
