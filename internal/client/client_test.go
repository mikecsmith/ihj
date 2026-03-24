package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClient_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("Authorization header is empty; want non-empty")
		}
		if err := json.NewEncoder(w).Encode(User{AccountID: "abc", DisplayName: "Alice"}); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token", WithMaxRetries(0))
	user, err := c.FetchMyself()
	if err != nil {
		t.Fatal(err)
	}
	if user.AccountID != "abc" || user.DisplayName != "Alice" {
		t.Errorf("FetchMyself() = %+v; want AccountID=abc, DisplayName=Alice", user)
	}
}

func TestClient_Get_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		if _, err := w.Write([]byte(`{"errorMessages":["Issue not found"]}`)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "token", WithMaxRetries(0))
	_, err := c.FetchMyself()
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("APIError.StatusCode = %d; want 404", apiErr.StatusCode)
	}
}

func TestClient_Retry_On429(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(429)
			if _, err := w.Write([]byte("rate limited")); err != nil {
				t.Errorf("writing response: %v", err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(User{AccountID: "ok", DisplayName: "OK"}); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "token", WithMaxRetries(3))
	user, err := c.FetchMyself()
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if user.AccountID != "ok" {
		t.Errorf("FetchMyself().AccountID = %q; want \"ok\"", user.AccountID)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
	}
}

func TestClient_NoRetry_On400(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(400)
		if _, err := w.Write([]byte("bad request")); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "token", WithMaxRetries(3))
	_, err := c.FetchMyself()
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 400)", atomic.LoadInt32(&attempts))
	}
}

func TestClient_SearchIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}

		var req SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request body: %v", err)
			return
		}
		if req.JQL != "project = FOO" {
			t.Errorf("SearchRequest.JQL = %q; want \"project = FOO\"", req.JQL)
		}

		if err := json.NewEncoder(w).Encode(SearchResponse{
			Issues: []Issue{
				{Key: "FOO-1", Fields: IssueFields{Summary: "First"}},
			},
			Total:  1,
			IsLast: true,
		}); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "token", WithMaxRetries(0))
	resp, err := c.SearchIssues(SearchRequest{JQL: "project = FOO", Fields: []string{"summary"}, MaxResults: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Issues) != 1 || resp.Issues[0].Key != "FOO-1" {
		t.Errorf("SearchIssues() = %v; want 1 issue with Key=FOO-1", resp.Issues)
	}
	if !resp.IsLast {
		t.Errorf("SearchIssues().IsLast = %v; want true", resp.IsLast)
	}
}

func TestClient_CreateIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}
		w.WriteHeader(201)
		if err := json.NewEncoder(w).Encode(CreatedIssue{ID: "10001", Key: "FOO-99", Self: "https://x/10001"}); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "token", WithMaxRetries(0))
	created, err := c.CreateIssue(map[string]any{"fields": map[string]any{"summary": "Test"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.Key != "FOO-99" {
		t.Errorf("CreateIssue().Key = %q; want \"FOO-99\"", created.Key)
	}
}

func TestClient_Put_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s; want PUT", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := New(srv.URL, "token", WithMaxRetries(0))
	err := c.AssignIssue("FOO-1", "account-123")
	if err != nil {
		t.Errorf("AssignIssue() = %v; want nil", err)
	}
}

func TestAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{429, true},
		{503, true},
		{400, false},
		{401, false},
		{404, false},
		{500, false},
	}
	for _, tt := range tests {
		e := &APIError{StatusCode: tt.code}
		if got := e.IsRetryable(); got != tt.want {
			t.Errorf("IsRetryable(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}
