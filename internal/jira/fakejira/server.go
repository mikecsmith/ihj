// Package fakejira provides an in-process, deterministic Jira REST
// endpoint suitable for demo mode and provider tests.
//
// A Server embeds *httptest.Server, wraps a mutable State, and routes
// Jira API requests to that State. The State exposes factories for
// issues/comments and seeded domain data (users, statuses, types,
// priorities, sprints) — all build from the same deterministic
// sequence counters so IDs are stable run-to-run.
//
// This package is the demo-mode backend: `ihj demo` constructs a
// Server, builds a core.Workspace pointing at Server.URL, and wires
// a real jira.Provider against it. That means demo mode exercises
// the same code path as production Jira — no separate provider
// implementation, no synthetic WorkItems.
package fakejira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

// Server wraps httptest.Server with a pointer to the mutable State so
// tests and callers can mutate the backing dataset during a run.
type Server struct {
	*httptest.Server
	State *State
}

// NewServer builds a Server backed by a freshly-seeded scrum State (DEMO project).
func NewServer() *Server {
	state := NewState()
	Seed(state)
	return NewServerWithState(state)
}

// NewKanbanServer builds a Server backed by a freshly-seeded kanban State (OPS project).
func NewKanbanServer() *Server {
	state := NewStateWithConfig(KanbanConfig())
	SeedKanban(state)
	return NewServerWithState(state)
}

// NewServerWithState builds a Server backed by the provided State.
// Useful for tests that want to pre-populate custom scenarios.
func NewServerWithState(state *State) *Server {
	s := &Server{State: state}
	s.Server = httptest.NewServer(http.HandlerFunc(s.route))
	return s
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	seg := strings.Split(strings.Trim(path, "/"), "/")

	switch {
	case r.Method == http.MethodGet && path == "/rest/api/3/myself":
		s.handleMyself(w)

	case r.Method == http.MethodGet && path == "/rest/api/3/field":
		s.handleFields(w)

	case r.Method == http.MethodGet && path == "/rest/api/3/status":
		s.handleStatuses(w)

	case r.Method == http.MethodPost && path == "/rest/api/3/search/jql":
		s.handleSearch(w, r)

	case r.Method == http.MethodPost && path == "/rest/api/3/issue":
		s.handleCreateIssue(w, r)

	case r.Method == http.MethodGet && len(seg) == 5 && seg[3] == "project":
		// /rest/api/3/project/{key}
		s.handleProject(w, seg[4])

	case r.Method == http.MethodGet && len(seg) == 7 && seg[3] == "issue" && seg[4] == "createmeta" && seg[6] == "issuetypes":
		// /rest/api/3/issue/createmeta/{project}/issuetypes
		s.handleCreateMetaTypes(w)

	case r.Method == http.MethodGet && len(seg) == 8 && seg[3] == "issue" && seg[4] == "createmeta" && seg[6] == "issuetypes":
		// /rest/api/3/issue/createmeta/{project}/issuetypes/{typeID}
		s.handleCreateMetaFields(w, r)

	case r.Method == http.MethodGet && strings.HasPrefix(path, "/rest/api/3/user/search"):
		s.handleUserSearch(w, r)

	case r.Method == http.MethodGet && len(seg) == 5 && seg[3] == "issue":
		// /rest/api/3/issue/{key}
		s.handleGetIssue(w, seg[4])

	case r.Method == http.MethodPut && len(seg) == 5 && seg[3] == "issue":
		s.handleUpdateIssue(w, r, seg[4])

	case r.Method == http.MethodGet && len(seg) == 6 && seg[3] == "issue" && seg[5] == "transitions":
		s.handleGetTransitions(w, seg[4])

	case r.Method == http.MethodPost && len(seg) == 6 && seg[3] == "issue" && seg[5] == "transitions":
		s.handleDoTransition(w, r, seg[4])

	case r.Method == http.MethodPost && len(seg) == 6 && seg[3] == "issue" && seg[5] == "comment":
		s.handleAddComment(w, r, seg[4])

	case r.Method == http.MethodGet && len(seg) == 6 && seg[3] == "issue" && seg[5] == "comment":
		s.handleGetComments(w, seg[4])

	case r.Method == http.MethodPut && len(seg) == 6 && seg[3] == "issue" && seg[5] == "assignee":
		s.handleAssignIssue(w, r, seg[4])

	case r.Method == http.MethodGet && path == "/rest/agile/1.0/board":
		s.handleListBoards(w, r)

	case r.Method == http.MethodGet && len(seg) == 6 && seg[1] == "agile" && seg[3] == "board" && seg[5] == "configuration":
		s.handleBoardConfig(w, seg[4])

	case r.Method == http.MethodGet && len(seg) == 6 && seg[1] == "agile" && seg[3] == "board" && seg[5] == "sprint":
		s.handleBoardSprints(w, r, seg[4])

	case r.Method == http.MethodPost && len(seg) == 5 && seg[1] == "agile" && seg[2] == "sprint" && seg[4] == "issue":
		s.handleAddToSprint(w, r, seg[3])

	case r.Method == http.MethodPost && path == "/rest/agile/1.0/backlog":
		s.handleBacklog(w, r)

	default:
		http.Error(w, fmt.Sprintf("fakejira: no route for %s %s", r.Method, path), http.StatusNotFound)
	}
}

// ── Handlers ─────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleMyself(w http.ResponseWriter) {
	u := s.State.Myself()
	if u == nil {
		http.Error(w, "no current user", http.StatusInternalServerError)
		return
	}
	writeJSON(w, wireUser{
		AccountID: u.AccountID, DisplayName: u.DisplayName, Email: u.Email, Active: true,
	})
}

func (s *Server) handleFields(w http.ResponseWriter) {
	// Minimal field registry — includes the custom fields the demo uses
	// so bootstrap's customfield discovery finds sane defaults.
	fields := []map[string]any{
		{"id": "summary", "name": "Summary"},
		{"id": "description", "name": "Description"},
		{"id": "issuetype", "name": "Issue Type"},
		{"id": "status", "name": "Status"},
		{"id": "priority", "name": "Priority"},
		{"id": "assignee", "name": "Assignee"},
		{"id": "reporter", "name": "Reporter"},
		{"id": "labels", "name": "Labels"},
		{"id": "components", "name": "Components"},
		{"id": "created", "name": "Created"},
		{"id": "updated", "name": "Updated"},
		{"id": CFStoryPoints, "name": "Story Points"},
		{"id": CFSprint, "name": "Sprint"},
	}
	writeJSON(w, fields)
}

func (s *Server) handleStatuses(w http.ResponseWriter) {
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	out := make([]wireStatus, 0, len(s.State.statuses))
	for _, st := range s.State.statuses {
		out = append(out, s.State.toWireStatus(st.ID))
	}
	writeJSON(w, out)
}

func (s *Server) handleProject(w http.ResponseWriter, key string) {
	if key != s.State.Config.ProjectKey {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	types := make([]wireIssueType, 0, len(s.State.types))
	for _, t := range s.State.types {
		types = append(types, wireIssueType{ID: t.ID, Name: t.Name, Subtask: t.Subtask})
	}
	writeJSON(w, map[string]any{
		"id":         "10000",
		"key":        s.State.Config.ProjectKey,
		"name":       s.State.Config.ProjectName,
		"issueTypes": types,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()

	var req struct {
		JQL           string   `json:"jql"`
		Fields        []string `json:"fields"`
		MaxResults    int      `json:"maxResults"`
		NextPageToken string   `json:"nextPageToken"`
	}
	_ = json.Unmarshal(body, &req)

	issues := s.State.Issues()
	issues = filterByJQL(s.State, issues, req.JQL)

	out := wireSearchResponse{IsLast: true, Total: len(issues)}
	for _, iss := range issues {
		s.State.mu.RLock()
		out.Issues = append(out.Issues, s.State.issueToWire(iss, req.Fields))
		s.State.mu.RUnlock()
	}
	writeJSON(w, out)
}

func (s *Server) handleGetIssue(w http.ResponseWriter, key string) {
	iss := s.State.Issue(key)
	if iss == nil {
		http.Error(w, "issue not found", http.StatusNotFound)
		return
	}
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	writeJSON(w, s.State.issueToWire(iss, nil))
}

func (s *Server) handleGetTransitions(w http.ResponseWriter, key string) {
	iss := s.State.Issue(key)
	if iss == nil {
		http.Error(w, "issue not found", http.StatusNotFound)
		return
	}
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	writeJSON(w, wireTransitionsResponse{Transitions: s.State.transitionsFor(iss)})
}

func (s *Server) handleDoTransition(w http.ResponseWriter, r *http.Request, key string) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	var req struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
	}
	_ = json.Unmarshal(body, &req)

	// Find target status from the transition ID we previously emitted.
	iss := s.State.Issue(key)
	if iss == nil {
		http.Error(w, "issue not found", http.StatusNotFound)
		return
	}
	s.State.mu.RLock()
	trans := s.State.transitionsFor(iss)
	s.State.mu.RUnlock()
	var targetID string
	for _, t := range trans {
		if t.ID == req.Transition.ID {
			targetID = t.To.ID
			break
		}
	}
	if targetID == "" {
		http.Error(w, "unknown transition", http.StatusBadRequest)
		return
	}
	_ = s.State.UpdateIssue(key, func(i *entIssue) { i.StatusID = targetID })
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	var req struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	iss := s.State.CreateIssue(func(i *entIssue) { applyFields(s.State, i, req.Fields) })
	writeJSON(w, wireCreatedIssue{ID: iss.ID, Key: iss.Key, Self: "/rest/api/3/issue/" + iss.Key})
}

func (s *Server) handleUpdateIssue(w http.ResponseWriter, r *http.Request, key string) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	var req struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.State.UpdateIssue(key, func(i *entIssue) { applyFields(s.State, i, req.Fields) }); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAssignIssue(w http.ResponseWriter, r *http.Request, key string) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	var req struct {
		AccountID *string `json:"accountId"`
	}
	_ = json.Unmarshal(body, &req)
	assignee := ""
	if req.AccountID != nil {
		assignee = *req.AccountID
	}
	if err := s.State.UpdateIssue(key, func(i *entIssue) { i.AssigneeID = assignee }); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request, key string) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	var req struct {
		Body any `json:"body"`
	}
	_ = json.Unmarshal(body, &req)
	if err := s.State.AppendComment(key, s.State.Me, req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleGetComments(w http.ResponseWriter, key string) {
	iss := s.State.Issue(key)
	if iss == nil {
		http.Error(w, "issue not found", http.StatusNotFound)
		return
	}
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	cp := wireCommentPage{
		Comments:   make([]wireComment, 0, len(iss.Comments)),
		MaxResults: len(iss.Comments), Total: len(iss.Comments),
	}
	for _, c := range iss.Comments {
		raw, _ := json.Marshal(c.Body)
		cp.Comments = append(cp.Comments, wireComment{
			ID: c.ID, Author: s.State.toWireUser(c.AuthorID), Body: raw,
			Created: fmtISO(c.Created), Updated: fmtISO(c.Created),
		})
	}
	writeJSON(w, cp)
}

func (s *Server) handleUserSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("query"))
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	var out []wireUser
	for _, u := range s.State.users {
		if strings.Contains(strings.ToLower(u.Email), query) || strings.Contains(strings.ToLower(u.DisplayName), query) {
			out = append(out, wireUser{
				AccountID: u.AccountID, DisplayName: u.DisplayName,
				Email: u.Email, Active: true,
			})
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListBoards(w http.ResponseWriter, r *http.Request) {
	cfg := s.State.Config
	key := r.URL.Query().Get("projectKeyOrId")
	if key != "" && key != cfg.ProjectKey {
		writeJSON(w, map[string]any{"values": []any{}})
		return
	}
	writeJSON(w, map[string]any{
		"values": []map[string]any{
			{"id": cfg.BoardID, "name": cfg.ProjectKey, "type": cfg.BoardType},
		},
	})
}

func (s *Server) handleBoardConfig(w http.ResponseWriter, idStr string) {
	cfg := s.State.Config
	id, _ := strconv.Atoi(idStr)
	if id != cfg.BoardID {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	columns := []map[string]any{}
	for _, st := range s.State.statuses {
		columns = append(columns, map[string]any{
			"name":     st.Name,
			"statuses": []map[string]string{{"id": st.ID}},
		})
	}
	writeJSON(w, map[string]any{
		"id":           cfg.BoardID,
		"name":         cfg.ProjectKey,
		"filter":       map[string]string{"id": "1"},
		"columnConfig": map[string]any{"columns": columns},
	})
}

func (s *Server) handleBoardSprints(w http.ResponseWriter, r *http.Request, boardIDStr string) {
	bid, _ := strconv.Atoi(boardIDStr)
	state := r.URL.Query().Get("state")
	sprints := s.State.SprintsByState(bid, state)
	out := wireSprintList{Values: make([]wireSprintRef, 0, len(sprints))}
	for _, sp := range sprints {
		out.Values = append(out.Values, wireSprintRef{ID: sp.ID, Name: sp.Name, State: sp.State})
	}
	writeJSON(w, out)
}

func (s *Server) handleAddToSprint(w http.ResponseWriter, r *http.Request, sprintIDStr string) {
	sid, _ := strconv.Atoi(sprintIDStr)
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	var req struct {
		Issues []string `json:"issues"`
	}
	_ = json.Unmarshal(body, &req)
	s.State.AddToSprint(sid, req.Issues)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBacklog(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	var req struct {
		Issues []string `json:"issues"`
	}
	_ = json.Unmarshal(body, &req)
	s.State.MoveToBacklog(req.Issues)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateMetaTypes(w http.ResponseWriter) {
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	out := wireCreateMetaIssueTypeList{Total: len(s.State.types)}
	for _, t := range s.State.types {
		out.IssueTypes = append(out.IssueTypes, wireCreateMetaIssueType{
			ID: t.ID, Name: t.Name, Subtask: t.Subtask,
		})
	}
	writeJSON(w, out)
}

func (s *Server) handleCreateMetaFields(w http.ResponseWriter, r *http.Request) {
	startAt, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
	s.State.mu.RLock()
	fields := s.State.createMetaFields()
	s.State.mu.RUnlock()
	total := len(fields)
	var page []wireCreateMetaField
	if startAt < total {
		page = fields[startAt:]
	}
	writeJSON(w, wireCreateMetaFieldList{
		Fields: page, StartAt: startAt, MaxResults: total, Total: total,
	})
}

// applyFields mutates an issue with the fields from a create/update payload.
// Only recognises the subset ihj's provider actually sends.
func applyFields(st *State, i *entIssue, fields map[string]any) {
	for k, v := range fields {
		switch k {
		case "summary":
			if s, ok := v.(string); ok {
				i.Summary = s
			}
		case "description":
			i.Description = v
		case "issuetype":
			if m, ok := v.(map[string]any); ok {
				if id, ok := m["id"].(string); ok {
					i.TypeID = id
				} else if name, ok := m["name"].(string); ok {
					for _, t := range st.types {
						if strings.EqualFold(t.Name, name) {
							i.TypeID = t.ID
							break
						}
					}
				}
			}
		case "priority":
			if m, ok := v.(map[string]any); ok {
				if id, ok := m["id"].(string); ok {
					i.PriorityID = id
				} else if name, ok := m["name"].(string); ok {
					for _, p := range st.priorities {
						if strings.EqualFold(p.Name, name) {
							i.PriorityID = p.ID
							break
						}
					}
				}
			}
		case "parent":
			if m, ok := v.(map[string]any); ok {
				if k, ok := m["key"].(string); ok {
					i.ParentKey = k
				}
			}
		case "labels":
			if arr, ok := v.([]any); ok {
				i.Labels = i.Labels[:0]
				for _, x := range arr {
					if s, ok := x.(string); ok {
						i.Labels = append(i.Labels, s)
					}
				}
			}
		case "components":
			if arr, ok := v.([]any); ok {
				i.Components = i.Components[:0]
				for _, x := range arr {
					if m, ok := x.(map[string]any); ok {
						if name, ok := m["name"].(string); ok {
							i.Components = append(i.Components, name)
						}
					}
				}
			}
		case "project":
			// no-op (single-project demo)
		default:
			// Custom fields — stash as-is.
			if strings.HasPrefix(k, "customfield_") {
				if i.Customs == nil {
					i.Customs = map[string]any{}
				}
				i.Customs[k] = v
			}
		}
	}
}
