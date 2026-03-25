// Package jira implements the Atlassian Jira provider for the application.
//
// It acts as an adapter between the raw REST API client and the universal
// domain model defined in the `work` package. Its primary responsibility is
// translating Jira-specific concepts (ADF descriptions, JQL, custom fields,
// and transitions) into backend-agnostic `work.WorkItem` structures.
//
// TODO(refactor): Implement the Universal Provider Interface
//   - Once `work.Provider` is formally defined (e.g., Create, Update, Fetch),
//     refactor these standalone functions into a `JiraAdapter` struct that
//     satisfies the interface, fully decoupling the `commands` package.
//
// TODO(refactor): Delegate Tree Assembly to the Domain
//   - BuildExportHierarchy currently builds the parent/child nested tree
//     itself. Once the `work` package provides a generic `BuildTree` helper,
//     this package should only be responsible for returning a flat slice of
//     items and a parent mapping.
//
// TODO(refactor): Populate the Flex Bucket
//   - Update the translation logic to map Jira's `customfield_XXXX` values
//     into the `work.WorkItem.Fields` map so that users can view and edit
//     backend-specific attributes seamlessly in the YAML.
package jira

import (
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/client"
)

// FilterTransitions orders API transitions according to config preference.
func FilterTransitions(transitions []client.Transition, allowed []string) []client.Transition {
	if len(allowed) == 0 {
		return transitions
	}
	var filtered []client.Transition
	for _, target := range allowed {
		for _, t := range transitions {
			if strings.EqualFold(t.Name, target) {
				filtered = append(filtered, t)
				break
			}
		}
	}
	return filtered
}

// FindTransitionID returns the transition ID matching a target status.
func FindTransitionID(transitions []client.Transition, target string) string {
	for _, t := range transitions {
		if strings.EqualFold(t.Name, target) || strings.EqualFold(t.To.Name, target) {
			return t.ID
		}
	}
	return ""
}

// PerformTransition fetches available transitions and executes the match.
func PerformTransition(c client.API, issueKey, targetStatus string) error {
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
func AssignToSprint(c client.API, boardID int, issueKey string) (bool, error) {
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
func FetchAllIssues(c client.API, jql string, formattedCF map[string]string) ([]client.Issue, error) {
	var all []client.Issue
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

// FetchIssueByKey performs a direct GET for a single issue.
func FetchIssueByKey(c client.API, issueKey string, formattedCF map[string]string) (*IssueView, error) {
	// 1. Call the direct FetchIssue endpoint we added to the client
	raw, err := c.FetchIssue(issueKey)
	if err != nil {
		return nil, fmt.Errorf("fetching issue %s: %w", issueKey, err)
	}

	registry := BuildRegistry([]client.Issue{*raw})

	view, ok := registry[issueKey]
	if !ok {
		return nil, fmt.Errorf("failed to process issue view for %s", issueKey)
	}

	return view, nil
}
