// Package jira implements the Atlassian Jira provider for the application.
//
// It acts as an adapter between the raw REST API client and the universal
// domain model defined in the core package. Its primary responsibility is
// translating Jira-specific concepts (ADF descriptions, JQL, custom fields,
// and transitions) into backend-agnostic core.WorkItem structures.
package jira

import (
	"fmt"
	"strings"
)

// FindTransitionID returns the transition ID matching a target status.
func FindTransitionID(transitions []Transition, target string) string {
	for _, t := range transitions {
		if strings.EqualFold(t.Name, target) || strings.EqualFold(t.To.Name, target) {
			return t.ID
		}
	}
	return ""
}

// PerformTransition fetches available transitions and executes the match.
func PerformTransition(c API, issueKey, targetStatus string) error {
	transitions, err := c.FetchTransitions(issueKey)
	if err != nil {
		return fmt.Errorf("fetching transitions for %s: %w", issueKey, err)
	}

	tid := FindTransitionID(transitions, targetStatus)
	if tid == "" {
		return fmt.Errorf("no valid transition to '%s' for %s", targetStatus, issueKey)
	}

	return c.DoTransition(issueKey, tid)
}

// AssignToSprint finds the active sprint and adds the issue.
// Returns false if no active sprint exists (not an error condition).
func AssignToSprint(c API, boardID int, issueKey string) (bool, error) {
	sprint, err := c.FetchActiveSprint(boardID)
	if err != nil {
		return false, fmt.Errorf("fetching active sprint: %w", err)
	}
	if sprint == nil {
		return false, nil
	}
	if err := c.AddToSprint(sprint.ID, []string{issueKey}); err != nil {
		return false, fmt.Errorf("adding %s to sprint %d: %w", issueKey, sprint.ID, err)
	}
	return true, nil
}

// FetchAllIssues handles paginated search, returning all matching issues.
func FetchAllIssues(c API, jql string, formattedCF map[string]string) ([]Issue, error) {
	var all []Issue
	nextToken := ""

	for {
		req := BuildSearchRequest(jql, formattedCF, nextToken)
		resp, err := c.SearchIssues(req)
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

