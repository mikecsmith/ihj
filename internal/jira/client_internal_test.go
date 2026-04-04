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

func TestClient_FetchCreateMetaIssueTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/createmeta/PROJ/issuetypes" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(createMetaIssueTypeList{
			IssueTypes: []createMetaIssueType{
				{ID: "10001", Name: "Task", Subtask: false},
				{ID: "10002", Name: "Sub-task", Subtask: true},
			},
			Total: 2,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	types, err := c.FetchCreateMetaIssueTypes(context.Background(), "PROJ")
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 2 {
		t.Fatalf("len = %d, want 2", len(types))
	}
	if types[0].Name != "Task" || types[0].ID != "10001" {
		t.Errorf("types[0] = %+v", types[0])
	}
	if !types[1].Subtask {
		t.Error("types[1].Subtask should be true")
	}
}

func TestClient_FetchCreateMetaFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/createmeta/PROJ/issuetypes/10001" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(createMetaFieldList{
			Fields: []createMetaField{
				{
					FieldID:  "priority",
					Key:      "priority",
					Name:     "Priority",
					Required: false,
					Schema:   fieldSchema{Type: "priority", System: "priority"},
					AllowedValues: json.RawMessage(`[
						{"id":"1","name":"Highest"},
						{"id":"2","name":"High"},
						{"id":"3","name":"Medium"}
					]`),
				},
				{
					FieldID:  "customfield_10016",
					Key:      "customfield_10016",
					Name:     "Story Points",
					Required: true,
					Schema:   fieldSchema{Type: "number", Custom: "com.atlassian.jira.plugin.system.customfieldtypes:float", CustomID: 10016},
				},
			},
			Total: 2,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	fields, err := c.FetchCreateMetaFields(context.Background(), "PROJ", "10001")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 {
		t.Fatalf("len = %d, want 2", len(fields))
	}
	if fields[0].Key != "priority" || fields[0].Schema.System != "priority" {
		t.Errorf("fields[0] = %+v", fields[0])
	}
	if !fields[1].Required || fields[1].Schema.CustomID != 10016 {
		t.Errorf("fields[1] = %+v", fields[1])
	}
	if fields[1].AllowedValues != nil {
		t.Error("fields[1].AllowedValues should be nil")
	}
}

func TestClient_FetchCreateMetaFields_Pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		switch startAt {
		case "0", "":
			json.NewEncoder(w).Encode(createMetaFieldList{
				Fields: []createMetaField{
					{FieldID: "summary", Key: "summary", Name: "Summary"},
					{FieldID: "priority", Key: "priority", Name: "Priority"},
				},
				StartAt:    0,
				MaxResults: 2,
				Total:      3,
			})
		case "2":
			json.NewEncoder(w).Encode(createMetaFieldList{
				Fields: []createMetaField{
					{FieldID: "customfield_10016", Key: "customfield_10016", Name: "Story Points"},
				},
				StartAt:    2,
				MaxResults: 2,
				Total:      3,
			})
		default:
			t.Errorf("unexpected startAt = %q", startAt)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	fields, err := c.FetchCreateMetaFields(context.Background(), "PROJ", "10001")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 {
		t.Fatalf("len = %d, want 3", len(fields))
	}
	if fields[0].Key != "summary" {
		t.Errorf("fields[0].Key = %q", fields[0].Key)
	}
	if fields[2].Key != "customfield_10016" {
		t.Errorf("fields[2].Key = %q", fields[2].Key)
	}
}

func TestClient_FetchCreateMetaFields_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"errorMessages":["Forbidden"]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	_, err := c.FetchCreateMetaFields(context.Background(), "PROJ", "10001")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*apiError)
	if !ok {
		t.Fatalf("expected *apiError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
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
