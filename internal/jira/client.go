package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// API is the interface for all Jira operations. The concrete Client implements
// it against the real REST API; MockClient implements it with in-memory data
// for demo mode and testing.
type API interface {
	SearchIssues(ctx context.Context, req searchRequest) (*searchResponse, error)
	FetchTransitions(ctx context.Context, issueKey string) ([]transition, error)
	DoTransition(ctx context.Context, issueKey, transitionID string) error
	FetchMyself(ctx context.Context) (*user, error)
	AssignIssue(ctx context.Context, issueKey, accountID string) error
	CreateIssue(ctx context.Context, payload map[string]any) (*createdIssue, error)
	UpdateIssue(ctx context.Context, issueKey string, payload map[string]any) error
	AddComment(ctx context.Context, issueKey string, adfBody map[string]any) error
	FetchActiveSprint(ctx context.Context, boardID int) (*sprint, error)
	FetchNextFutureSprint(ctx context.Context, boardID int) (*sprint, error)
	AddToSprint(ctx context.Context, sprintID int, issueKeys []string) error
	MoveToBacklog(ctx context.Context, issueKeys []string) error
	FetchIssue(ctx context.Context, issueKey string) (*issue, error)
	FetchBoardConfig(ctx context.Context, boardID int) (*boardConfiguration, error)
	FetchFilter(ctx context.Context, filterID string) (*jiraFilter, error)
	FetchFields(ctx context.Context) ([]fieldDefinition, error)
	FetchStatuses(ctx context.Context) ([]status, error)
	FetchProject(ctx context.Context, projectKey string) (*project, error)
	FetchBoardsForProject(ctx context.Context, projectKey string) ([]agileBoard, error)
	SearchUsers(ctx context.Context, query string) ([]user, error)
}

// Compile-time check that *Client implements API.
var _ API = (*Client)(nil)

// Client handles all communication with the Jira REST API.
type Client struct {
	Server     string
	token      string
	httpClient *http.Client
	maxRetries int
}

// New creates a Jira REST API client for the given server and auth token.
func New(server, token string) *Client {
	return &Client{
		Server:     server,
		token:      token,
		maxRetries: 3,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// apiError represents a non-2xx response from Jira.
type apiError struct {
	StatusCode int
	Body       string
	Method     string
	Path       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("jira %s %s: HTTP %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

func (e *apiError) IsRetryable() bool {
	return e.StatusCode == 429 || e.StatusCode == 503
}

func (c *Client) SearchIssues(ctx context.Context, req searchRequest) (*searchResponse, error) {
	var resp searchResponse
	if err := c.post(ctx, "/rest/api/3/search/jql", req, &resp); err != nil {
		return nil, err
	}
	if resp.NextPageToken == "" {
		resp.IsLast = true
	}
	return &resp, nil
}

func (c *Client) FetchTransitions(ctx context.Context, issueKey string) ([]transition, error) {
	var resp transitionsResponse
	if err := c.get(ctx, fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), &resp); err != nil {
		return nil, err
	}
	return resp.Transitions, nil
}

func (c *Client) DoTransition(ctx context.Context, issueKey, transitionID string) error {
	payload := map[string]any{"transition": map[string]any{"id": transitionID}}
	return c.postNoResponse(ctx, fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), payload)
}

func (c *Client) FetchMyself(ctx context.Context) (*user, error) {
	var u user
	if err := c.get(ctx, "/rest/api/3/myself", &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) AssignIssue(ctx context.Context, issueKey, accountID string) error {
	// Jira API requires {"accountId": null} to unassign, not {"accountId": ""}.
	var payload map[string]any
	if accountID == "" {
		payload = map[string]any{"accountId": nil}
	} else {
		payload = map[string]any{"accountId": accountID}
	}
	return c.put(ctx, fmt.Sprintf("/rest/api/3/issue/%s/assignee", issueKey), payload)
}

func (c *Client) CreateIssue(ctx context.Context, payload map[string]any) (*createdIssue, error) {
	var resp createdIssue
	if err := c.post(ctx, "/rest/api/3/issue", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateIssue(ctx context.Context, issueKey string, payload map[string]any) error {
	return c.put(ctx, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
}

func (c *Client) AddComment(ctx context.Context, issueKey string, adfBody map[string]any) error {
	return c.postNoResponse(ctx, fmt.Sprintf("/rest/api/3/issue/%s/comment", issueKey),
		map[string]any{"body": adfBody})
}

func (c *Client) FetchIssue(ctx context.Context, issueKey string) (*issue, error) {
	var iss issue
	if err := c.get(ctx, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), &iss); err != nil {
		return nil, err
	}
	return &iss, nil
}

func (c *Client) FetchActiveSprint(ctx context.Context, boardID int) (*sprint, error) {
	var resp sprintList
	if err := c.get(ctx, fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=active", boardID), &resp); err != nil {
		return nil, err
	}
	if len(resp.Values) == 0 {
		return nil, nil
	}
	return &resp.Values[0], nil
}

// FetchNextFutureSprint returns the earliest future sprint (lowest ID) for
// a board. When multiple future sprints exist, the lowest-ID sprint is the
// one created first — typically the next sprint to be started.
func (c *Client) FetchNextFutureSprint(ctx context.Context, boardID int) (*sprint, error) {
	var resp sprintList
	if err := c.get(ctx, fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=future", boardID), &resp); err != nil {
		return nil, err
	}
	if len(resp.Values) == 0 {
		return nil, nil
	}
	// Pick the lowest-ID sprint — earliest created, next to start.
	best := resp.Values[0]
	for _, s := range resp.Values[1:] {
		if s.ID < best.ID {
			best = s
		}
	}
	return &best, nil
}

func (c *Client) AddToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	return c.postNoResponse(ctx, fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID),
		map[string]any{"issues": issueKeys})
}

// MoveToBacklog removes issues from any sprint, placing them in the backlog.
func (c *Client) MoveToBacklog(ctx context.Context, issueKeys []string) error {
	return c.postNoResponse(ctx, "/rest/agile/1.0/backlog",
		map[string]any{"issues": issueKeys})
}

func (c *Client) FetchBoardConfig(ctx context.Context, boardID int) (*boardConfiguration, error) {
	var cfg boardConfiguration
	if err := c.get(ctx, fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Client) FetchFilter(ctx context.Context, filterID string) (*jiraFilter, error) {
	var f jiraFilter
	if err := c.get(ctx, fmt.Sprintf("/rest/api/3/filter/%s", filterID), &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (c *Client) FetchFields(ctx context.Context) ([]fieldDefinition, error) {
	var fields []fieldDefinition
	if err := c.get(ctx, "/rest/api/3/field", &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func (c *Client) FetchStatuses(ctx context.Context) ([]status, error) {
	var statuses []status
	if err := c.get(ctx, "/rest/api/3/status", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) FetchProject(ctx context.Context, projectKey string) (*project, error) {
	var p project
	if err := c.get(ctx, fmt.Sprintf("/rest/api/3/project/%s", projectKey), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Client) FetchBoardsForProject(ctx context.Context, projectKey string) ([]agileBoard, error) {
	var resp agileBoardList
	if err := c.get(ctx, fmt.Sprintf("/rest/agile/1.0/board?projectKeyOrId=%s", projectKey), &resp); err != nil {
		return nil, err
	}
	return resp.Values, nil
}

// SearchUsers finds Jira users matching the given query (typically an email).
func (c *Client) SearchUsers(ctx context.Context, query string) ([]user, error) {
	var users []user
	if err := c.get(ctx, fmt.Sprintf("/rest/api/3/user/search?query=%s&maxResults=10", query), &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) get(ctx context.Context, path string, dest any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, dest)
}

func (c *Client) post(ctx context.Context, path string, payload, dest any) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, dest)
}

func (c *Client) postNoResponse(ctx context.Context, path string, payload any) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, nil)
}

func (c *Client) put(ctx context.Context, path string, payload any) error {
	req, err := c.newRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, nil)
}

func (c *Client) newRequest(ctx context.Context, method, path string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.Server+path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+c.token)
	if payload != nil || method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) doWithRetry(req *http.Request, dest any) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			time.Sleep(backoff)

			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return fmt.Errorf("recreating request body: %w", err)
				}
				req.Body = body
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response: %w", err)
			continue
		}

		if resp.StatusCode >= 400 {
			// Sanitize non-JSON responses (HTML from proxies/WAFs).
			body := string(bodyBytes)
			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, "application/json") && len(body) > 200 {
				body = body[:200] + "... (truncated non-JSON response)"
			}
			apiErr := &apiError{
				StatusCode: resp.StatusCode,
				Body:       body,
				Method:     req.Method,
				Path:       req.URL.Path,
			}
			if apiErr.IsRetryable() {
				lastErr = apiErr
				continue
			}
			return apiErr
		}

		if len(bodyBytes) == 0 || dest == nil {
			return nil
		}

		if err := json.Unmarshal(bodyBytes, dest); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
