package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClient_AuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Basic test-token" {
			t.Errorf("Authorization = %q; want \"Basic test-token\"", auth)
		}
		json.NewEncoder(w).Encode(user{AccountID: "abc", DisplayName: "Alice"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	_, err := c.FetchMyself(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_Retry_On429(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(429)
			w.Write([]byte("rate limited"))
			return
		}
		json.NewEncoder(w).Encode(user{AccountID: "ok", DisplayName: "OK"})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	u, err := c.FetchMyself(context.Background())
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if u.AccountID != "ok" {
		t.Errorf("FetchMyself().AccountID = %q; want \"ok\"", u.AccountID)
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
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	_, err := c.FetchMyself(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 400)", atomic.LoadInt32(&attempts))
	}
}

func TestClient_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"errorMessages":["Issue not found"]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	_, err := c.FetchMyself(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*apiError)
	if !ok {
		t.Fatalf("expected *apiError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d; want 404", apiErr.StatusCode)
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
		e := &apiError{StatusCode: tt.code}
		if got := e.IsRetryable(); got != tt.want {
			t.Errorf("IsRetryable(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}
