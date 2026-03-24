package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
)

func TestAssign_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/myself"):
			_ = json.NewEncoder(w).Encode(client.User{AccountID: "acc-1", DisplayName: "Me"})
		case strings.Contains(r.URL.Path, "/assignee"):
			if r.Method != http.MethodPut {
				t.Errorf("method = %s; want PUT", r.Method)
			}
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	ui := &MockUI{}
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))
	app.CacheDir = t.TempDir()

	err := Assign(app, "FOO-1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if !ui.HasNotification("Jira Updated") {
		t.Errorf("HasNotification(\"Jira Updated\") = false; want true")
	}
}

func TestAssign_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/myself") {
			_ = json.NewEncoder(w).Encode(client.User{AccountID: "acc-1"})
			return
		}
		w.WriteHeader(403)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	ui := &MockUI{}
	app := testApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))
	app.CacheDir = t.TempDir()

	err := Assign(app, "FOO-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !ui.HasNotification("Jira Error") {
		t.Errorf("HasNotification(\"Jira Error\") = false; want true")
	}
}
