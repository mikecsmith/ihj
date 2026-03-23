package client

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
	SearchIssues(req SearchRequest) (*SearchResponse, error)
	FetchTransitions(issueKey string) ([]Transition, error)
	DoTransition(issueKey, transitionID string) error
	FetchMyself() (*User, error)
	AssignIssue(issueKey, accountID string) error
	CreateIssue(payload map[string]any) (*CreatedIssue, error)
	UpdateIssue(issueKey string, payload map[string]any) error
	AddComment(issueKey string, adfBody map[string]any) error
	FetchActiveSprint(boardID int) (*Sprint, error)
	AddToSprint(sprintID int, issueKeys []string) error
	FetchIssue(issueKey string) (*Issue, error)
	FetchBoardConfig(boardID int) (*BoardConfiguration, error)
	FetchFilter(filterID string) (*Filter, error)
	FetchFields() ([]FieldDefinition, error)
	FetchStatuses() ([]Status, error)
	FetchProject(projectKey string) (*Project, error)
	FetchBoardsForProject(projectKey string) ([]AgileBoard, error)
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

type Option func(*Client)

func WithTimeout(d time.Duration) Option     { return func(c *Client) { c.httpClient.Timeout = d } }
func WithContext(ctx context.Context) Option { return func(c *Client) { c.ctx = ctx } }
func WithHTTPClient(hc *http.Client) Option  { return func(c *Client) { c.httpClient = hc } }
func WithMaxRetries(n int) Option            { return func(c *Client) { c.maxRetries = n } }

// APIError represents a non-2xx response from Jira.
type APIError struct {
	StatusCode int
	Body       string
	Method     string
	Path       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("jira %s %s: HTTP %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

func (e *APIError) IsRetryable() bool {
	return e.StatusCode == 429 || e.StatusCode == 503
}

// ──────────────────────────────────────────────────────────────
// Typed API methods — each returns concrete types, not map[string]any
// ──────────────────────────────────────────────────────────────

func (c *Client) SearchIssues(req SearchRequest) (*SearchResponse, error) {
	var resp SearchResponse
	if err := c.post("/rest/api/3/search/jql", req, &resp); err != nil {
		return nil, err
	}
	if resp.NextPageToken == "" {
		resp.IsLast = true
	}
	return &resp, nil
}

func (c *Client) FetchTransitions(issueKey string) ([]Transition, error) {
	var resp TransitionsResponse
	if err := c.get(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), &resp); err != nil {
		return nil, err
	}
	return resp.Transitions, nil
}

func (c *Client) DoTransition(issueKey, transitionID string) error {
	payload := map[string]any{"transition": map[string]any{"id": transitionID}}
	return c.postNoResponse(fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), payload)
}

func (c *Client) FetchMyself() (*User, error) {
	var user User
	if err := c.get("/rest/api/3/myself", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) AssignIssue(issueKey, accountID string) error {
	return c.put(fmt.Sprintf("/rest/api/3/issue/%s/assignee", issueKey),
		map[string]string{"accountId": accountID})
}

func (c *Client) CreateIssue(payload map[string]any) (*CreatedIssue, error) {
	var resp CreatedIssue
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

func (c *Client) FetchIssue(issueKey string) (*Issue, error) {
	var iss Issue
	if err := c.get(fmt.Sprintf("/rest/api/3/issue/%s", issueKey), &iss); err != nil {
		return nil, err
	}
	return &iss, nil
}

func (c *Client) FetchActiveSprint(boardID int) (*Sprint, error) {
	var resp SprintList
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

func (c *Client) FetchBoardConfig(boardID int) (*BoardConfiguration, error) {
	var cfg BoardConfiguration
	if err := c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Client) FetchFilter(filterID string) (*Filter, error) {
	var f Filter
	if err := c.get(fmt.Sprintf("/rest/api/3/filter/%s", filterID), &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (c *Client) FetchFields() ([]FieldDefinition, error) {
	var fields []FieldDefinition
	if err := c.get("/rest/api/3/field", &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func (c *Client) FetchStatuses() ([]Status, error) {
	var statuses []Status
	if err := c.get("/rest/api/3/status", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func (c *Client) FetchProject(projectKey string) (*Project, error) {
	var p Project
	if err := c.get(fmt.Sprintf("/rest/api/3/project/%s", projectKey), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Client) FetchBoardsForProject(projectKey string) ([]AgileBoard, error) {
	var resp AgileBoardList
	if err := c.get(fmt.Sprintf("/rest/agile/1.0/board?projectKeyOrId=%s", projectKey), &resp); err != nil {
		return nil, err
	}
	return resp.Values, nil
}

// ──────────────────────────────────────────────────────────────
// Internal HTTP plumbing
// ──────────────────────────────────────────────────────────────

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
		resp.Body.Close()
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
			apiErr := &APIError{
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
