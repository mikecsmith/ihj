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

// assignToSprint finds the active sprint and adds the issue.
// Returns false if no active sprint exists (not an error condition).
func assignToSprint(ctx context.Context, c API, boardID int, issueKey string) (bool, error) {
	s, err := c.FetchActiveSprint(ctx, boardID)
	if err != nil {
		return false, fmt.Errorf("fetching active sprint: %w", err)
	}
	if s == nil {
		return false, nil
	}
	if err := c.AddToSprint(ctx, s.ID, []string{issueKey}); err != nil {
		return false, fmt.Errorf("adding %s to sprint %d: %w", issueKey, s.ID, err)
	}
	return true, nil
}

// fetchAllIssues handles paginated search, returning all matching issues.
func fetchAllIssues(ctx context.Context, c API, jql string, formattedCF map[string]string) ([]issue, error) {
	var all []issue
	nextToken := ""

	for {
		req := buildSearchRequest(jql, formattedCF, nextToken)
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
