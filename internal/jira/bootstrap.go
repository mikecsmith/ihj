package jira

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/mikecsmith/ihj/internal/core"
)

// Prompter is the subset of user interaction needed by bootstrap.
// BubbleTeaUI satisfies this implicitly.
type Prompter interface {
	Select(title string, options []string) (int, error)
	Notify(title, message string)
	PromptText(prompt string) (string, error)
}

// Bootstrap scaffolds a workspace config by querying the Jira API for board,
// status, type, and custom field definitions. serverURL is the Jira
// instance URL (e.g. https://company.atlassian.net); if empty and this
// is a fresh config, the user is prompted for it.
func Bootstrap(ctx context.Context, client API, ui Prompter, out io.Writer, projectKey, serverURL string, existingWorkspaceCount int) error {
	projectKey = strings.ToUpper(projectKey)

	ui.Notify("Bootstrap", fmt.Sprintf("Searching for boards linked to %s...", projectKey))

	boards, err := client.FetchBoardsForProject(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("fetching boards: %w", err)
	}
	if len(boards) == 0 {
		return fmt.Errorf("no boards found for project %s", projectKey)
	}

	sort.Slice(boards, func(i, j int) bool {
		return strings.ToLower(boards[i].Name) < strings.ToLower(boards[j].Name)
	})

	options := make([]string, len(boards))
	for i, b := range boards {
		options[i] = fmt.Sprintf("%s (ID: %d)", b.Name, b.ID)
	}

	choice, err := ui.Select(fmt.Sprintf("Select board for %s", projectKey), options)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &core.CancelledError{Operation: "bootstrap"}
	}

	selected := boards[choice]
	boardSlug := strings.ToLower(strings.ReplaceAll(selected.Name, " ", "_"))

	ui.Notify("Bootstrap", "Fetching board configuration...")
	boardCfg, err := client.FetchBoardConfig(ctx, selected.ID)
	if err != nil {
		return fmt.Errorf("fetching board config: %w", err)
	}

	ui.Notify("Bootstrap", "Fetching base JQL filter...")
	filterData, err := client.FetchFilter(ctx, boardCfg.Filter.ID)
	if err != nil {
		return fmt.Errorf("fetching filter: %w", err)
	}
	baseJQL := filterData.JQL

	ui.Notify("Bootstrap", "Fetching status definitions...")
	allStatuses, err := client.FetchStatuses(ctx)
	if err != nil {
		return fmt.Errorf("fetching statuses: %w", err)
	}
	statusMap := make(map[string]status)
	for _, s := range allStatuses {
		statusMap[s.ID] = s
	}

	var columnNames, visibleStatuses, doneStatuses []string
	for _, col := range boardCfg.ColumnConfig.Columns {
		columnNames = append(columnNames, col.Name)
		for _, s := range col.Statuses {
			if st, ok := statusMap[s.ID]; ok {
				visibleStatuses = append(visibleStatuses, st.Name)
				if st.StatusCategory.Key == "done" {
					doneStatuses = append(doneStatuses, st.Name)
				}
			}
		}
	}

	statusJQL := quoteJoin(visibleStatuses)
	doneJQL := quoteJoin(doneStatuses)
	if doneJQL == "" {
		doneJQL = `"Done"`
	}

	ui.Notify("Bootstrap", "Discovering custom fields...")
	allFields, err := client.FetchFields(ctx)
	if err != nil {
		return fmt.Errorf("fetching fields: %w", err)
	}
	cfMap := discoverCustomFields(allFields)

	ui.Notify("Bootstrap", "Interpolating JQL variables...")
	baseJQL, teamUUID := interpolateBootstrapJQL(baseJQL, cfMap)

	ui.Notify("Bootstrap", fmt.Sprintf("Mapping issue types for %s...", projectKey))
	proj, err := client.FetchProject(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("fetching project: %w", err)
	}
	typesList := buildTypesList(proj.IssueTypes)

	// Resolve server URL — prompt if not provided on a fresh config.
	if existingWorkspaceCount == 0 && serverURL == "" {
		var err error
		serverURL, err = ui.PromptText("Jira Server URL (e.g., https://company.atlassian.net)")
		if err != nil || serverURL == "" {
			return fmt.Errorf("server URL is required for bootstrap")
		}
	}

	// Derive a server alias from the URL hostname.
	serverAlias := ServerAliasFromURL(serverURL)

	// Build the workspace YAML payload.
	wsPayload := map[string]any{
		"server":      serverAlias,
		"name":        selected.Name,
		"project_key": projectKey,
		"board_id":    selected.ID,
	}
	if teamUUID != "" {
		wsPayload["team_uuid"] = teamUUID
	}
	wsPayload["jql"] = baseJQL
	wsPayload["filters"] = map[string]string{
		"all":    "",
		"active": fmt.Sprintf("status IN (%s) AND (statusCategory != Done OR (statusCategory = Done AND status CHANGED TO (%s) AFTER -2w))", statusJQL, doneJQL),
		"me":     "assignee = currentUser() AND statusCategory != Done",
	}
	wsPayload["statuses"] = columnNames
	wsPayload["types"] = typesList
	wsPayload["custom_fields"] = cfMap

	scaffold := make(map[string]any)

	// Add server definition.
	scaffold["servers"] = map[string]any{
		serverAlias: map[string]any{
			"provider": core.ProviderJira,
			"url":      serverURL,
		},
	}

	if existingWorkspaceCount == 0 {
		scaffold["default_workspace"] = boardSlug
		scaffold["editor"] = "vim"
	}

	scaffold["workspaces"] = map[string]any{boardSlug: wsPayload}

	yamlBytes, err := yaml.Marshal(scaffold)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}

	if _, err := fmt.Fprint(out, string(yamlBytes)); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// ServerAliasFromURL derives a human-readable server alias from a URL.
// For example, "https://mycompany.atlassian.net" becomes "mycompany-atlassian-net".
func ServerAliasFromURL(serverURL string) string {
	u, err := url.Parse(serverURL)
	if err != nil || u.Host == "" {
		// Fallback: strip protocol and replace dots/slashes.
		alias := strings.TrimPrefix(serverURL, "https://")
		alias = strings.TrimPrefix(alias, "http://")
		alias = strings.ReplaceAll(alias, ".", "-")
		alias = strings.TrimRight(alias, "/")
		return alias
	}
	return strings.ReplaceAll(u.Hostname(), ".", "-")
}

func discoverCustomFields(fields []fieldDefinition) map[string]any {
	cfMap := make(map[string]any)
	var teamCandidates []int

	for _, f := range fields {
		if !strings.HasPrefix(f.ID, "customfield_") {
			continue
		}

		idStr := strings.TrimPrefix(f.ID, "customfield_")
		fid, err := strconv.Atoi(idStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse ID for field %q (%s): %v\n", f.Name, f.ID, err)
			continue
		}

		name := strings.ToLower(f.Name)
		switch name {
		case "team":
			teamCandidates = append(teamCandidates, fid)
		case "epic name":
			cfMap["epic_name"] = fid
		case "epic link":
			cfMap["epic_link"] = fid
		}
	}

	set := false
	if slices.Contains(teamCandidates, 15000) {
		cfMap["team"] = 15000
		set = true
	}
	if !set && len(teamCandidates) > 0 {
		cfMap["team"] = teamCandidates[0]
	} else if !set {
		cfMap["team"] = "TODO_FIND_TEAM_ID"
	}

	if _, ok := cfMap["epic_name"]; !ok {
		cfMap["epic_name"] = "TODO_FIND_EPIC_NAME_ID"
	}
	if _, ok := cfMap["epic_link"]; !ok {
		cfMap["epic_link"] = "TODO_FIND_EPIC_LINK_ID"
	}
	return cfMap
}

func interpolateBootstrapJQL(jql string, cfMap map[string]any) (string, string) {
	var teamUUID string
	if teamID, ok := cfMap["team"].(int); ok {
		re := regexp.MustCompile(
			fmt.Sprintf(`(?i)(?:cf\[%d\]|customfield_%d)\s*(?:=|in)\s*\(?\s*([a-zA-Z0-9\-]+)\s*\)?`, teamID, teamID),
		)
		if m := re.FindStringSubmatch(jql); len(m) > 1 {
			teamUUID = m[1]
			jql = re.ReplaceAllString(jql, `{team} = "{team_uuid}"`)
		}
	}
	projectRe := regexp.MustCompile(`(?i)project\s*(?:=|in)\s*\(?\s*\d+\s*\)?`)
	jql = projectRe.ReplaceAllString(jql, `project = "{project_key}"`)
	return jql, teamUUID
}

type bootstrapType struct {
	ID          int    `yaml:"id"`
	Name        string `yaml:"name"`
	Order       int    `yaml:"order"`
	Color       string `yaml:"color"`
	HasChildren bool   `yaml:"has_children"`
}

func buildTypesList(issueTypes []issueType) []bootstrapType {
	known := map[string]struct {
		order int
		color string
	}{
		"initiative": {10, "cyan"}, "epic": {20, "magenta"},
		"story": {30, "blue"}, "task": {30, "default"},
		"bug": {30, "red"}, "sub-task": {40, "white"},
	}

	var result []bootstrapType
	seen := make(map[string]bool)
	for _, t := range issueTypes {
		lower := strings.ToLower(t.Name)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		match, ok := known[lower]
		if !ok {
			match = struct {
				order int
				color string
			}{99, "default"}
		}
		tid, err := strconv.Atoi(t.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: non-integer issue type ID found for %q: %s\n", t.Name, t.ID)
			continue
		}
		result = append(result, bootstrapType{
			ID: tid, Name: t.Name, Order: match.order,
			Color: match.color, HasChildren: !t.Subtask,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Order < result[j].Order })
	return result
}

func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf(`"%s"`, s)
	}
	return strings.Join(quoted, ", ")
}
