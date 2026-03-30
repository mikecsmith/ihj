package commands_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/testutil"
)

func TestComment_EmptyAbort(t *testing.T) {
	ui := &testutil.MockUI{InputTextReturn: "   "}
	ws := testutil.NewTestSession(ui)
	ws.Provider = &testutil.MockProvider{}

	err := commands.Comment(context.Background(), ws, "FOO-1")
	if !commands.IsCancelled(err) {
		t.Errorf("expected CancelledError, got %v", err)
	}
}

func TestComment_Success(t *testing.T) {
	ui := &testutil.MockUI{InputTextReturn: "This is my comment."}
	mp := &testutil.MockProvider{}
	ws := testutil.NewTestSession(ui)
	ws.Provider = mp

	err := commands.Comment(context.Background(), ws, "FOO-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ui.HasNotification("Comment") {
		t.Errorf("hasNotification(\"Comment\") = false; want true")
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
	ui := &testutil.MockUI{InputTextReturn: "A comment"}
	mp := &testutil.MockProvider{CommentErr: fmt.Errorf("network error")}
	ws := testutil.NewTestSession(ui)
	ws.Provider = mp

	err := commands.Comment(context.Background(), ws, "FOO-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !ui.HasNotification("Error") {
		t.Errorf("hasNotification(\"Error\") = false; want true")
	}
}
