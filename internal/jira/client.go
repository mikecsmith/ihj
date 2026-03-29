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
	SearchIssues(req searchRequest) (*searchResponse, error)
	FetchTransitions(issueKey string) ([]transition, error)
	DoTransition(issueKey, transitionID string) error
	FetchMyself() (*user, error)
	AssignIssue(issueKey, accountID string) error
	CreateIssue(payload map[string]any) (*createdIssue, error)
	UpdateIssue(issueKey string, payload map[string]any) error
	AddComment(issueKey string, adfBody map[string]any) error
	FetchActiveSprint(boardID int) (*sprint, error)
	AddToSprint(sprintID int, issueKeys []string) error
	FetchIssue(issueKey string) (*issue, error)
	FetchBoardConfig(boardID int) (*boardConfiguration, error)
	FetchFilter(filterID string) (*jiraFilter, error)
	FetchFields() ([]fieldDefinition, error)
	FetchStatuses() ([]status, error)
	FetchProject(projectKey string) (*project, error)
	FetchBoardsForProject(projectKey string) ([]agileBoard, error)
	SearchUsers(query string) ([]user, error)
}

// Compile-time check that *Client implements API.
var _ API = (*Client)(nil)

// Client handles all communication with the Jira REST API.
type Client struct {
	Server     string
	token      string
	httpClient *http.Client
	ctx        context.Context
	maxRetries int
}

// New creates a Jira REST API client for the given server and auth token.
func New(server, token string, opts ...Option) *Client {
	c := &Client{
		Server:     server,
		token:      token,
		ctx:        context.Background(),
		maxRetries: 3,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option configures a Client.
type Option func(*Client)

// WithContext sets the base context for all HTTP requests.
func WithContext(ctx context.Context) Option { return func(c *Client) { c.ctx = ctx } }

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


func (c *Client) SearchIssues(req searchRequest) (*searchResponse, error) {
	var resp searchResponse
	if err := c.post("/rest/api/3/search/jql", req, &resp); err != nil {
		return nil, err
	}
	if resp.NextPageToken == "" {
		resp.IsLast = true
	}
	return &resp, nil
}

func (c *Client) FetchTransitions(issueKey string) ([]transition, error) {
	var resp transitionsResponse
	if err := c.get(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), &resp); err != nil {
		return nil, err
	}
	return resp.Transitions, nil
}

func (c *Client) DoTransition(issueKey, transitionID string) error {
	payload := map[string]any{"transition": map[string]any{"id": transitionID}}
	return c.postNoResponse(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), payload)
}

func (c *Client) FetchMyself() (*user, error) {
	var u user
	if err := c.get("/rest/api/3/myself", &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) AssignIssue(issueKey, accountID string) error {
	return c.put(fmt.Sprintf("/rest/api/3/issue/%s/assignee", issueKey),
		map[string]string{"accountId": accountID})
}

func (c *Client) CreateIssue(payload map[string]any) (*createdIssue, error) {
	var resp createdIssue
	if err := c.post("/rest/api/3/issue", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateIssue(issueKey string, payload map[string]any) error {
	return c.put(fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
}

func (c *Client) AddComment(issueKey string, adfBody map[string]any) error {
	return c.postNoResponse(fmt.Sprintf("/rest/api/3/issue/%s/comment", issueKey),
		map[string]any{"body": adfBody})
}

func (c *Client) FetchIssue(issueKey string) (*issue, error) {
	var iss issue
	if err := c.get(fmt.Sprintf("/rest/api/3/issue/%s", issueKey), &iss); err != nil {
		return nil, err
	}
	return &iss, nil
}

func (c *Client) FetchActiveSprint(boardID int) (*sprint, error) {
	var resp sprintList
	if err := c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=active", boardID), &resp); err != nil {
		return nil, err
	}
	if len(resp.Values) == 0 {
		return nil, nil
	}
	return &resp.Values[0], nil
}

func (c *Client) AddToSprint(sprintID int, issueKeys []string) error {
	return c.postNoResponse(fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID),
		map[string]any{"issues": issueKeys})
}

func (c *Client) FetchBoardConfig(boardID int) (*boardConfiguration, error) {
	var cfg boardConfiguration
	if err := c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Client) FetchFilter(filterID string) (*jiraFilter, error) {
	var f jiraFilter
	if err := c.get(fmt.Sprintf("/rest/api/3/filter/%s", filterID), &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (c *Client) FetchFields() ([]fieldDefinition, error) {
	var fields []fieldDefinition
	if err := c.get("/rest/api/3/field", &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func (c *Client) FetchStatuses() ([]status, error) {
	var statuses []status
	if err := c.get("/rest/api/3/status", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) FetchProject(projectKey string) (*project, error) {
	var p project
	if err := c.get(fmt.Sprintf("/rest/api/3/project/%s", projectKey), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Client) FetchBoardsForProject(projectKey string) ([]agileBoard, error) {
	var resp agileBoardList
	if err := c.get(fmt.Sprintf("/rest/agile/1.0/board?projectKeyOrId=%s", projectKey), &resp); err != nil {
		return nil, err
	}
	return resp.Values, nil
}

// SearchUsers finds Jira users matching the given query (typically an email).
func (c *Client) SearchUsers(query string) ([]user, error) {
	var users []user
	if err := c.get(fmt.Sprintf("/rest/api/3/user/search?query=%s&maxResults=10", query), &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) get(path string, dest any) error {
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, dest)
}

func (c *Client) post(path string, payload, dest any) error {
	req, err := c.newRequest(http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, dest)
}

func (c *Client) postNoResponse(path string, payload any) error {
	req, err := c.newRequest(http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, nil)
}

func (c *Client) put(path string, payload any) error {
	req, err := c.newRequest(http.MethodPut, path, payload)
	if err != nil {
		return err
	}
	return c.doWithRetry(req, nil)
}

func (c *Client) newRequest(method, path string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(c.ctx, method, c.Server+path, body)
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
