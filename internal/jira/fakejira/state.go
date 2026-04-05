package fakejira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mikecsmith/ihj/internal/testutil/factory"
)

// Project/board identifiers for the two demo scenarios.
const (
	ProjectKey       = "DEMO" // scrum project
	BoardID          = 1
	BoardType        = "scrum"
	KanbanProjectKey = "OPS" // kanban project
	KanbanBoardID    = 2
	KanbanBoardType  = "kanban"
)

// Custom field IDs used by the demo scenario.
const (
	CFStoryPoints = "customfield_10016"
	CFSprint      = "customfield_10020"
)

// Config parameterises the project/board a State represents. A single
// fakejira.Server hosts exactly one project — `ihj jira demo` wires up
// both a scrum and a kanban server and exposes each as its own workspace.
type Config struct {
	ProjectKey  string
	ProjectName string
	BoardID     int
	BoardType   string // "scrum" or "kanban"
}

// ScrumConfig returns the default scrum configuration (DEMO project).
func ScrumConfig() Config {
	return Config{ProjectKey: ProjectKey, ProjectName: "Demo Project", BoardID: BoardID, BoardType: BoardType}
}

// KanbanConfig returns the kanban configuration (OPS project).
func KanbanConfig() Config {
	return Config{ProjectKey: KanbanProjectKey, ProjectName: "Ops Support", BoardID: KanbanBoardID, BoardType: KanbanBoardType}
}

// State holds the mutable demo graph. Every API response is projected
// from this struct — mutations flow through the State's methods, which
// guard every access with a single lock.
type State struct {
	mu sync.RWMutex

	// Project/board this State represents.
	Config Config

	// Current authenticated user (for /myself).
	Me string // AccountID

	users      map[string]*entUser
	statuses   []*entStatus
	types      []*entIssueType
	priorities []*entPriority
	sprints    map[int]*entSprint
	issues     map[string]*entIssue

	// Factories used to mint future IDs deterministically.
	issueFactory   *factory.Factory[entIssue]
	commentFactory *factory.Factory[entComment]
}

// NewState builds a new empty scrum-project state with factories initialised.
func NewState() *State { return NewStateWithConfig(ScrumConfig()) }

// NewStateWithConfig builds a new empty state for the given project config.
func NewStateWithConfig(cfg Config) *State {
	s := &State{
		Config:  cfg,
		users:   make(map[string]*entUser),
		sprints: make(map[int]*entSprint),
		issues:  make(map[string]*entIssue),
	}
	s.issueFactory = factory.New(func(seq int64) entIssue {
		return entIssue{
			Key: issueKey(cfg.ProjectKey, seq),
			ID:  fmt.Sprintf("%d", 10000+seq),
		}
	})
	s.commentFactory = factory.New(func(seq int64) entComment {
		return entComment{ID: fmt.Sprintf("%d", 20000+seq)}
	})
	return s
}

// ── Reads ────────────────────────────────────────────────────────────────

// User returns the user with the given accountId (may be nil).
func (s *State) User(id string) *entUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[id]
}

// Myself returns the currently authenticated user.
func (s *State) Myself() *entUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[s.Me]
}

// StatusByID returns the status matching id, or nil.
func (s *State) StatusByID(id string) *entStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, st := range s.statuses {
		if st.ID == id {
			return st
		}
	}
	return nil
}

// StatusByName returns the status whose name matches (case-insensitive).
func (s *State) StatusByName(name string) *entStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, st := range s.statuses {
		if strings.EqualFold(st.Name, name) {
			return st
		}
	}
	return nil
}

// TypeByID returns the issue type with the given id, or nil.
func (s *State) TypeByID(id string) *entIssueType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.types {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// PriorityByID returns the priority with the given id, or nil.
func (s *State) PriorityByID(id string) *entPriority {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.priorities {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// Issue returns the issue with the given key, or nil.
func (s *State) Issue(key string) *entIssue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.issues[key]
}

// Sprint returns the sprint with the given id, or nil.
func (s *State) Sprint(id int) *entSprint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sprints[id]
}

// SprintsByState returns sprints in the given state, sorted by ID.
func (s *State) SprintsByState(boardID int, state string) []*entSprint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*entSprint
	for _, sp := range s.sprints {
		if sp.BoardID == boardID && sp.State == state {
			out = append(out, sp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Issues returns all issues sorted by key.
func (s *State) Issues() []*entIssue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*entIssue, 0, len(s.issues))
	for _, iss := range s.issues {
		out = append(out, iss)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// ── Mutations ────────────────────────────────────────────────────────────

// CreateIssue persists a new issue with a freshly minted key.
func (s *State) CreateIssue(apply func(*entIssue)) *entIssue {
	s.mu.Lock()
	defer s.mu.Unlock()
	iss := s.issueFactory.Build()
	iss.Created = time.Now().UTC()
	iss.Updated = iss.Created
	if apply != nil {
		apply(&iss)
	}
	s.issues[iss.Key] = &iss
	return &iss
}

// UpdateIssue applies a mutator to the named issue under lock.
func (s *State) UpdateIssue(key string, apply func(*entIssue)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	iss, ok := s.issues[key]
	if !ok {
		return fmt.Errorf("issue %s not found", key)
	}
	apply(iss)
	iss.Updated = time.Now().UTC()
	return nil
}

// AddToSprint moves a set of issues into the given sprint.
func (s *State) AddToSprint(sprintID int, keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		if iss, ok := s.issues[k]; ok {
			iss.SprintID = sprintID
			iss.Updated = time.Now().UTC()
		}
	}
}

// MoveToBacklog clears the sprint assignment of a set of issues.
func (s *State) MoveToBacklog(keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		if iss, ok := s.issues[k]; ok {
			iss.SprintID = 0
			iss.Updated = time.Now().UTC()
		}
	}
}

// AppendComment adds a comment (as ADF) to an issue.
func (s *State) AppendComment(key string, authorID string, adfBody any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	iss, ok := s.issues[key]
	if !ok {
		return fmt.Errorf("issue %s not found", key)
	}
	c := s.commentFactory.Build()
	c.AuthorID = authorID
	c.Body = adfBody
	c.Created = time.Now().UTC()
	iss.Comments = append(iss.Comments, c)
	iss.Updated = c.Created
	return nil
}

// ── Wire conversions ─────────────────────────────────────────────────────

func (s *State) toWireUser(id string) *wireUser {
	u := s.users[id]
	if u == nil {
		return nil
	}
	return &wireUser{
		AccountID:   u.AccountID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Active:      true,
	}
}

func (s *State) toWireStatus(id string) wireStatus {
	for _, st := range s.statuses {
		if st.ID == id {
			return wireStatus{
				ID:   st.ID,
				Name: st.Name,
				StatusCategory: wireStatusCategory{
					ID:   statusCategoryID(st.CategoryKey),
					Key:  st.CategoryKey,
					Name: statusCategoryName(st.CategoryKey),
				},
			}
		}
	}
	return wireStatus{}
}

func (s *State) toWireType(id string) wireIssueType {
	for _, t := range s.types {
		if t.ID == id {
			return wireIssueType{ID: t.ID, Name: t.Name, Subtask: t.Subtask}
		}
	}
	return wireIssueType{}
}

func (s *State) toWirePriority(id string) wirePriority {
	for _, p := range s.priorities {
		if p.ID == id {
			return wirePriority{ID: p.ID, Name: p.Name}
		}
	}
	return wirePriority{}
}

// issueToWire builds a wire-format issue with the requested fields populated.
// When requestedFields is nil or contains "*all", every field is included.
func (s *State) issueToWire(iss *entIssue, requestedFields []string) wireIssue {
	wantAll := len(requestedFields) == 0
	for _, f := range requestedFields {
		if f == "*all" {
			wantAll = true
			break
		}
	}
	want := func(name string) bool {
		if wantAll {
			return true
		}
		for _, f := range requestedFields {
			if f == name {
				return true
			}
		}
		return false
	}

	fields := map[string]json.RawMessage{}

	set := func(k string, v any) {
		raw, _ := json.Marshal(v)
		fields[k] = raw
	}

	if want("summary") {
		set("summary", iss.Summary)
	}
	if want("description") {
		if iss.Description != nil {
			set("description", iss.Description)
		} else {
			fields["description"] = json.RawMessage("null")
		}
	}
	if want("issuetype") {
		set("issuetype", s.toWireType(iss.TypeID))
	}
	if want("status") {
		set("status", s.toWireStatus(iss.StatusID))
	}
	if want("priority") {
		set("priority", s.toWirePriority(iss.PriorityID))
	}
	if want("assignee") {
		set("assignee", s.toWireUser(iss.AssigneeID))
	}
	if want("reporter") {
		set("reporter", s.toWireUser(iss.ReporterID))
	}
	if want("labels") {
		set("labels", iss.Labels)
	}
	if want("components") {
		comps := make([]wireComponent, 0, len(iss.Components))
		for i, name := range iss.Components {
			comps = append(comps, wireComponent{ID: fmt.Sprintf("%d", 30000+i), Name: name})
		}
		set("components", comps)
	}
	if want("comment") {
		cp := wireCommentPage{
			Comments:   make([]wireComment, 0, len(iss.Comments)),
			MaxResults: len(iss.Comments),
			Total:      len(iss.Comments),
		}
		for _, c := range iss.Comments {
			body, _ := json.Marshal(c.Body)
			cp.Comments = append(cp.Comments, wireComment{
				ID:      c.ID,
				Author:  s.toWireUser(c.AuthorID),
				Body:    body,
				Created: fmtISO(c.Created),
				Updated: fmtISO(c.Created),
			})
		}
		set("comment", cp)
	}
	if want("created") {
		set("created", fmtISO(iss.Created))
	}
	if want("updated") {
		set("updated", fmtISO(iss.Updated))
	}
	if want("parent") && iss.ParentKey != "" {
		if parent, ok := s.issues[iss.ParentKey]; ok {
			set("parent", wireParentRef{
				Key: parent.Key,
				ID:  parent.ID,
				Fields: &wireParentSubFields{
					Summary:   parent.Summary,
					Status:    s.toWireStatus(parent.StatusID),
					IssueType: s.toWireType(parent.TypeID),
				},
			})
		}
	}

	// Custom fields — story_points as a value, sprint as an array of sprint refs.
	if want(CFStoryPoints) || wantAll {
		if v, ok := iss.Customs[CFStoryPoints]; ok {
			set(CFStoryPoints, v)
		}
	}
	if want(CFSprint) || wantAll {
		if iss.SprintID != 0 {
			if sp, ok := s.sprints[iss.SprintID]; ok {
				set(CFSprint, []wireSprintRef{{
					ID:    sp.ID,
					Name:  sp.Name,
					State: sp.State,
				}})
			}
		}
	}

	// Any other customs the seed assigned.
	for k, v := range iss.Customs {
		if k == CFStoryPoints {
			continue
		}
		if want(k) || wantAll {
			set(k, v)
		}
	}

	return wireIssue{
		Key:    iss.Key,
		ID:     iss.ID,
		Self:   "/rest/api/3/issue/" + iss.Key,
		Fields: fields,
	}
}

// transitionsFor computes the transitions currently available to an issue.
// In fakejira every non-current status is reachable.
func (s *State) transitionsFor(iss *entIssue) []wireTransition {
	out := make([]wireTransition, 0, len(s.statuses))
	nextID := 100
	for _, st := range s.statuses {
		if st.ID == iss.StatusID {
			continue
		}
		out = append(out, wireTransition{
			ID:   fmt.Sprintf("%d", nextID),
			Name: st.Name,
			To:   s.toWireStatus(st.ID),
		})
		nextID++
	}
	return out
}

// createMetaFields returns the wire-format createmeta fields for a type.
// The set is identical across types in the demo scenario.
func (s *State) createMetaFields() []wireCreateMetaField {
	priorityValues := make([]map[string]any, 0, len(s.priorities))
	for _, p := range s.priorities {
		priorityValues = append(priorityValues, map[string]any{"id": p.ID, "name": p.Name})
	}

	return []wireCreateMetaField{
		{
			FieldID: "summary", Key: "summary", Name: "Summary", Required: true,
			Schema: wireFieldSchema{Type: "string", System: "summary"},
		},
		{
			FieldID: "priority", Key: "priority", Name: "Priority", Required: false,
			Schema:        wireFieldSchema{Type: "priority", System: "priority"},
			AllowedValues: priorityValues,
		},
		{
			FieldID: "assignee", Key: "assignee", Name: "Assignee", Required: false,
			Schema: wireFieldSchema{Type: "user", System: "assignee"},
		},
		{
			FieldID: "labels", Key: "labels", Name: "Labels", Required: false,
			Schema: wireFieldSchema{Type: "array", Items: "string", System: "labels"},
		},
		{
			FieldID: CFStoryPoints, Key: "story_points", Name: "Story Points", Required: false,
			Schema: wireFieldSchema{
				Type: "number", Custom: "com.atlassian.jira.plugin.system.customfieldtypes:float",
				CustomID: 10016,
			},
		},
		{
			FieldID: CFSprint, Key: "sprint", Name: "Sprint", Required: false,
			Schema: wireFieldSchema{
				Type: "array", Items: "json",
				Custom:   "com.pyxis.greenhopper.jira:gh-sprint",
				CustomID: 10020,
			},
		},
	}
}
