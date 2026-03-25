package commands

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/document"
)

func TestComment_EmptyAbort(t *testing.T) {
	ui := &MockUI{EditTextReturn: "   "}
	app := NewTestApp(ui)

	err := Comment(app, "FOO-1")
	if !IsCancelled(err) {
		t.Errorf("expected CancelledError, got %v", err)
	}
}

func TestComment_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/comment") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	ui := &MockUI{EditTextReturn: "This is my comment."}
	app := NewTestApp(ui)
	app.Client = client.New(srv.URL, "token", client.WithMaxRetries(0))

	err := Comment(app, "FOO-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ui.HasNotification("Jira Comment") {
		t.Errorf("HasNotification(\"Jira Comment\") = false; want true")
	}
}

func TestParseComment(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantParas int // expected number of top-level AST children (-1 to skip check)
	}{
		{"simple text", "Hello world", false, 1},
		{"with formatting", "**bold** and *italic*", false, 1},
		{"empty string", "", false, 0},
		{"multiline", "Line 1\n\nLine 2", false, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adf, ast, err := ParseComment(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseComment(%q) = nil error; want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseComment(%q) error = %v; want nil", tt.input, err)
			}
			if adf == nil {
				t.Errorf("ParseComment(%q) adf = nil; want non-nil", tt.input)
			}
			if adf["type"] != "doc" {
				t.Errorf("ParseComment(%q) adf[\"type\"] = %v; want \"doc\"", tt.input, adf["type"])
			}
			if ast == nil {
				t.Fatalf("ParseComment(%q) ast = nil; want non-nil", tt.input)
			}
			if ast.Type != document.NodeDoc {
				t.Errorf("ParseComment(%q) ast.Type = %v; want NodeDoc", tt.input, ast.Type)
			}
			if tt.wantParas >= 0 && len(ast.Children) != tt.wantParas {
				t.Errorf("ParseComment(%q) ast children = %d; want %d", tt.input, len(ast.Children), tt.wantParas)
			}
		})
	}
}
