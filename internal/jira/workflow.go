package jira

import (
	"context"
	"fmt"
	"strings"
)

// findTransitionID returns the transition ID matching a target status.
func findTransitionID(transitions []transition, target string) string {
	for _, t := range transitions {
		if strings.EqualFold(t.Name, target) || strings.EqualFold(t.To.Name, target) {
			return t.ID
		}
	}
	return ""
}

// performTransition fetches available transitions and executes the match.
func performTransition(ctx context.Context, c API, issueKey, targetStatus string) error {
	transitions, err := c.FetchTransitions(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("fetching transitions for %s: %w", issueKey, err)
	}

	tid := findTransitionID(transitions, targetStatus)
	if tid == "" {
		return fmt.Errorf("no valid transition to '%s' for %s", targetStatus, issueKey)
	}

	return c.DoTransition(ctx, issueKey, tid)
}

// sprintAssign assigns an issue to the active or next future sprint, or
// removes it from any sprint ("none" → backlog). Returns an error if no
// matching sprint exists — callers decide whether to treat this as fatal
// or a warning.
func sprintAssign(ctx context.Context, c API, boardID int, issueKey, target string) error {
	var s *sprint
	var err error

	switch target {
	case "none":
		if err := c.MoveToBacklog(ctx, []string{issueKey}); err != nil {
			return fmt.Errorf("moving %s to backlog: %w", issueKey, err)
		}
		return nil
	case "active":
		s, err = c.FetchActiveSprint(ctx, boardID)
		if err != nil {
			return fmt.Errorf("fetching active sprint: %w", err)
		}
		if s == nil {
			return fmt.Errorf("no active sprint on board %d", boardID)
		}
	case "future":
		s, err = c.FetchNextFutureSprint(ctx, boardID)
		if err != nil {
			return fmt.Errorf("fetching future sprints: %w", err)
		}
		if s == nil {
			return fmt.Errorf("no future sprint on board %d", boardID)
		}
	default:
		return fmt.Errorf("unknown sprint target %q (expected \"active\", \"future\", or \"none\")", target)
	}

	if err := c.AddToSprint(ctx, s.ID, []string{issueKey}); err != nil {
		return fmt.Errorf("adding %s to sprint %q (%d): %w", issueKey, s.Name, s.ID, err)
	}
	return nil
}

// fetchAllIssues handles paginated search, returning all matching issues.
// extraFields are additional Jira field IDs to include in the response.
func fetchAllIssues(ctx context.Context, c API, jql string, formattedCF map[string]string, extraFields []string) ([]issue, error) {
	var all []issue
	nextToken := ""

	for {
		req := buildSearchRequest(jql, formattedCF, extraFields, nextToken)
		resp, err := c.SearchIssues(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("searching issues: %w", err)
		}

		all = append(all, resp.Issues...)

		if resp.IsLast || resp.NextPageToken == "" {
			break
		}
		nextToken = resp.NextPageToken
	}

	return all, nil
}
