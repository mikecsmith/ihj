package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MockClient implements API with in-memory data for demo mode and testing.
// All mutations (transition, comment, assign, create) operate on the in-memory
// store and are immediately visible in subsequent reads.
type MockClient struct {
	mu     sync.RWMutex
	issues map[string]*Issue // keyed by issue key
	nextID int              // auto-increment for CreateIssue

	// Configurable behaviors.
	ProjectKey  string
	Transitions []string      // allowed transition names (statuses)
	Users       []User        // available users (for FetchMyself, etc.)
	Latency     time.Duration // simulated API latency (0 = no delay)
}

var _ API = (*MockClient)(nil)

func (m *MockClient) simulateLatency() {
	if m.Latency > 0 {
		time.Sleep(m.Latency)
	}
}

// NewMockClient creates a mock client pre-loaded with issues.
func NewMockClient(issues []Issue, transitions []string, projectKey string) *MockClient {
	m := &MockClient{
		issues:      make(map[string]*Issue, len(issues)),
		Transitions: transitions,
		ProjectKey:  projectKey,
		nextID:      1000,
		Users: []User{
			{AccountID: "demo-user-1", DisplayName: "Demo User", Active: true},
		},
	}
	for i := range issues {
		m.issues[issues[i].Key] = &issues[i]
	}
	return m
}

// AddIssue adds or replaces an issue in the mock store (for testing).
func (m *MockClient) AddIssue(iss Issue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	issCopy := iss
	m.issues[iss.Key] = &issCopy
}

// SearchIssues returns in-memory issues. Supports basic "key = X" JQL filtering;
// all other JQL patterns return everything.
func (m *MockClient) SearchIssues(req SearchRequest) (*SearchResponse, error) {
	m.simulateLatency()
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple "key = DEMO-1" filter for edit mode.
	if strings.HasPrefix(req.JQL, "key = ") || strings.HasPrefix(req.JQL, "key=") {
		keyVal := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(req.JQL, "key = "), "key="))
		if iss, ok := m.issues[keyVal]; ok {
			return &SearchResponse{Issues: []Issue{*iss}, IsLast: true}, nil
		}
		return &SearchResponse{IsLast: true}, nil
	}

	all := make([]Issue, 0, len(m.issues))
	for _, iss := range m.issues {
		all = append(all, *iss)
	}
	return &SearchResponse{
		Issues: all,
		IsLast: true,
	}, nil
}

// FetchTransitions returns transitions derived from the configured status list.
func (m *MockClient) FetchTransitions(_ string) ([]Transition, error) {
	m.simulateLatency()
	var transitions []Transition
	for i, name := range m.Transitions {
		transitions = append(transitions, Transition{
			ID:   fmt.Sprintf("%d", i+1),
			Name: name,
			To:   Status{Name: name},
		})
	}
	return transitions, nil
}

// DoTransition changes an issue's status in memory.
func (m *MockClient) DoTransition(issueKey, transitionID string) error {
	m.simulateLatency()
	m.mu.Lock()
	defer m.mu.Unlock()

	iss, ok := m.issues[issueKey]
	if !ok {
		return &APIError{StatusCode: 404, Method: "POST", Path: issueKey, Body: "issue not found"}
	}

	for i, name := range m.Transitions {
		if fmt.Sprintf("%d", i+1) == transitionID {
			iss.Fields.Status = Status{Name: name}
			return nil
		}
	}
	return fmt.Errorf("unknown transition ID: %s", transitionID)
}

// FetchMyself returns the first configured demo user.
func (m *MockClient) FetchMyself() (*User, error) {
	m.simulateLatency()
	if len(m.Users) == 0 {
		return &User{AccountID: "demo", DisplayName: "Demo User", Active: true}, nil
	}
	u := m.Users[0] // copy to avoid returning pointer to slice element
	return &u, nil
}

// AssignIssue sets the assignee on an in-memory issue.
func (m *MockClient) AssignIssue(issueKey, accountID string) error {
	m.simulateLatency()
	m.mu.Lock()
	defer m.mu.Unlock()

	iss, ok := m.issues[issueKey]
	if !ok {
		return &APIError{StatusCode: 404, Method: "PUT", Path: issueKey, Body: "issue not found"}
	}

	// Find user by account ID.
	for _, u := range m.Users {
		if u.AccountID == accountID {
			uCopy := u
			iss.Fields.Assignee = &uCopy
			return nil
		}
	}
	iss.Fields.Assignee = &User{AccountID: accountID, DisplayName: "Unknown"}
	return nil
}

// CreateIssue adds a new issue to the in-memory store.
func (m *MockClient) CreateIssue(payload map[string]any) (*CreatedIssue, error) {
	m.simulateLatency()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	key := fmt.Sprintf("%s-%d", m.ProjectKey, m.nextID)

	now := time.Now().Format("2006-01-02T15:04:05.000-0700")
	iss := &Issue{
		Key: key,
		Fields: IssueFields{
			Summary:   "New Issue",
			Status:    Status{Name: m.Transitions[0]},
			IssueType: IssueType{Name: "Task", ID: "3"},
			Created:   now,
			Updated:   now,
		},
	}

	if fields, ok := payload["fields"].(map[string]any); ok {
		if s, ok := fields["summary"].(string); ok {
			iss.Fields.Summary = s
		}
		if it, ok := fields["issuetype"].(map[string]any); ok {
			if name, ok := it["name"].(string); ok {
				iss.Fields.IssueType.Name = name
			}
		}
		if p, ok := fields["priority"].(map[string]any); ok {
			if name, ok := p["name"].(string); ok {
				iss.Fields.Priority = Priority{Name: name}
			}
		}
		if desc, ok := fields["description"].(map[string]any); ok {
			if raw, err := json.Marshal(desc); err == nil {
				iss.Fields.Description = raw
			}
		}
		if parent, ok := fields["parent"].(map[string]any); ok {
			if pk, ok := parent["key"].(string); ok {
				iss.Fields.Parent = &ParentRef{Key: pk}
			}
		}
		// Set reporter to demo user.
		if len(m.Users) > 0 {
			u := m.Users[0]
			iss.Fields.Reporter = &u
		}
	}

	m.issues[key] = iss

	return &CreatedIssue{Key: key}, nil
}

// UpdateIssue modifies an in-memory issue's fields from the payload.
func (m *MockClient) UpdateIssue(issueKey string, payload map[string]any) error {
	m.simulateLatency()
	m.mu.Lock()
	defer m.mu.Unlock()

	iss, ok := m.issues[issueKey]
	if !ok {
		return &APIError{StatusCode: 404, Method: "PUT", Path: issueKey, Body: "issue not found"}
	}

	if fields, ok := payload["fields"].(map[string]any); ok {
		if s, ok := fields["summary"].(string); ok {
			iss.Fields.Summary = s
		}
		if p, ok := fields["priority"].(map[string]any); ok {
			if name, ok := p["name"].(string); ok {
				iss.Fields.Priority = Priority{Name: name}
			}
		}
		if desc, ok := fields["description"].(map[string]any); ok {
			if raw, err := json.Marshal(desc); err == nil {
				iss.Fields.Description = raw
			}
		}
		if parent, ok := fields["parent"].(map[string]any); ok {
			if key, ok := parent["key"].(string); ok {
				iss.Fields.Parent = &ParentRef{Key: key}
			}
		}
	}
	iss.Fields.Updated = time.Now().Format("2006-01-02T15:04:05.000-0700")
	return nil
}

// AddComment appends a comment to an in-memory issue.
func (m *MockClient) AddComment(issueKey string, adfBody map[string]any) error {
	m.simulateLatency()
	m.mu.Lock()
	defer m.mu.Unlock()

	iss, ok := m.issues[issueKey]
	if !ok {
		return &APIError{StatusCode: 404, Method: "POST", Path: issueKey, Body: "issue not found"}
	}

	user, _ := m.FetchMyself()
	bodyJSON, _ := json.Marshal(adfBody)

	comment := Comment{
		Author:  user,
		Created: time.Now().Format("2006-01-02T15:04:05.000-0700"),
		Body:    json.RawMessage(bodyJSON),
	}

	if iss.Fields.Comment == nil {
		iss.Fields.Comment = &CommentPage{}
	}
	iss.Fields.Comment.Comments = append(iss.Fields.Comment.Comments, comment)
	return nil
}

// FetchActiveSprint returns a fake sprint.
func (m *MockClient) FetchActiveSprint(_ int) (*Sprint, error) {
	m.simulateLatency()
	return &Sprint{ID: 1, Name: "Demo Sprint", State: "active"}, nil
}

// AddToSprint is a no-op in demo mode.
func (m *MockClient) AddToSprint(_ int, _ []string) error {
	m.simulateLatency()
	return nil
}

// FetchBoardConfig returns a minimal board configuration.
func (m *MockClient) FetchBoardConfig(_ int) (*BoardConfiguration, error) {
	return &BoardConfiguration{}, nil
}

// FetchFilter returns a dummy filter.
func (m *MockClient) FetchFilter(_ string) (*Filter, error) {
	return &Filter{JQL: "project = " + m.ProjectKey}, nil
}

// FetchFields returns an empty field list.
func (m *MockClient) FetchFields() ([]FieldDefinition, error) {
	return nil, nil
}

// FetchStatuses returns statuses derived from configured transitions.
func (m *MockClient) FetchStatuses() ([]Status, error) {
	var statuses []Status
	for _, name := range m.Transitions {
		statuses = append(statuses, Status{Name: name})
	}
	return statuses, nil
}

// FetchProject returns a minimal project.
func (m *MockClient) FetchProject(_ string) (*Project, error) {
	return &Project{Key: m.ProjectKey, Name: "Demo Project"}, nil
}

// FetchBoardsForProject returns a single demo board.
func (m *MockClient) FetchBoardsForProject(_ string) ([]AgileBoard, error) {
	return []AgileBoard{{ID: 1, Name: "Demo Board"}}, nil
}
