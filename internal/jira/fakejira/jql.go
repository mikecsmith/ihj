package fakejira

import (
	"regexp"
	"strings"
)

// filterByJQL applies a coarse approximation of Jira's JQL filtering
// against the in-memory issue list. It recognises just enough syntax to
// back the demo workspace filters (`active`, `backlog`, `me`, status
// checks) — this is not a JQL parser, only a pragmatic subset.
//
// Supported clauses (combined with AND):
//   - sprint IS EMPTY / sprint NOT IN openSprints() → issues with no
//     sprint or a future sprint.
//   - sprint IN openSprints() AND sprint NOT IN futureSprints() →
//     issues on an active sprint.
//   - assignee = "X" / assignee = currentUser() → match assignee.
//   - status = "X" / status != "X" → match status name (case-insensitive).
//   - project = "X" → drop if keys don't match the State's project.
func filterByJQL(s *State, issues []*entIssue, jql string) []*entIssue {
	if strings.TrimSpace(jql) == "" {
		return issues
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	lower := strings.ToLower(jql)

	// Project clause — fakejira serves a single project, so mismatches
	// empty the result set entirely.
	if key := extractQuoted(lower, `project\s*=\s*["']?([a-z0-9_-]+)["']?`); key != "" {
		if !strings.EqualFold(key, s.Config.ProjectKey) {
			return nil
		}
	}

	wantEmptySprint := strings.Contains(lower, "sprint is empty") ||
		strings.Contains(lower, "sprint not in opensprints")
	wantActiveSprint := strings.Contains(lower, "sprint in opensprints") &&
		!wantEmptySprint

	assignee := extractQuoted(lower, `assignee\s*=\s*["']?([a-z0-9_.@-]+)["']?`)
	if assignee == "currentuser()" || assignee == "currentuser" {
		assignee = strings.ToLower(s.Me)
	}

	statusEq := extractQuoted(lower, `status\s*=\s*["']?([a-z0-9_ -]+?)["']?(?:\s|$|and|or|\))`)
	statusNe := extractQuoted(lower, `status\s*!=\s*["']?([a-z0-9_ -]+?)["']?(?:\s|$|and|or|\))`)

	out := issues[:0:0]
	for _, iss := range issues {
		if wantEmptySprint {
			// accept no sprint or a future sprint
			if iss.SprintID != 0 {
				sp, ok := s.sprints[iss.SprintID]
				if !ok || sp.State != "future" {
					continue
				}
			}
		}
		if wantActiveSprint {
			if iss.SprintID == 0 {
				continue
			}
			sp, ok := s.sprints[iss.SprintID]
			if !ok || sp.State != "active" {
				continue
			}
		}
		if assignee != "" && strings.ToLower(iss.AssigneeID) != assignee {
			continue
		}
		if statusEq != "" {
			st := s.statusByIDUnlocked(iss.StatusID)
			if st == nil || !strings.EqualFold(strings.TrimSpace(statusEq), st.Name) {
				continue
			}
		}
		if statusNe != "" {
			st := s.statusByIDUnlocked(iss.StatusID)
			if st != nil && strings.EqualFold(strings.TrimSpace(statusNe), st.Name) {
				continue
			}
		}
		out = append(out, iss)
	}
	return out
}

// extractQuoted runs a regex that captures a single group and returns
// the captured substring, lowered. Empty string when no match.
func extractQuoted(s, pattern string) string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// statusByIDUnlocked is the lock-free variant used by filterByJQL
// (which runs while the caller already holds the read lock via Issues()).
func (s *State) statusByIDUnlocked(id string) *entStatus {
	for _, st := range s.statuses {
		if st.ID == id {
			return st
		}
	}
	return nil
}
