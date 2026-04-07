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
// instance URL (e.g. https://company.atlassian.net); serverAlias is the
// config key for the server (e.g. "dev-jira"). If serverAlias is empty,
// one is derived from the URL.
func Bootstrap(ctx context.Context, client API, ui Prompter, out io.Writer, projectKey, serverURL, serverAlias string, existingWorkspaceCount int) error {
	projectKey = strings.ToUpper(projectKey)

	board, wsName, wsSlug, err := selectBoard(ctx, client, ui, projectKey)
	if err != nil {
		return err
	}

	discovery, err := discoverWorkspaceContext(ctx, client, ui, board, projectKey)
	if err != nil {
		return err
	}

	// Resolve server URL — prompt if not provided on a fresh config.
	if existingWorkspaceCount == 0 && serverURL == "" {
		serverURL, err = ui.PromptText("Jira Server URL (e.g., https://company.atlassian.net)")
		if err != nil || serverURL == "" {
			return fmt.Errorf("server URL is required for bootstrap")
		}
	}
	if serverAlias == "" {
		serverAlias = ServerAliasFromURL(serverURL)
	}

	return writeWorkspaceScaffold(out, wsName, wsSlug, projectKey, serverURL, serverAlias, existingWorkspaceCount, board, discovery)
}

// boardSelection holds the result of the board selection step.
type boardSelection struct {
	ID   int
	Name string
	Type string
}

// workspaceDiscovery holds all API-derived context needed to generate
// a workspace config scaffold.
type workspaceDiscovery struct {
	BaseJQL   string
	TeamUUID  string
	StatusJQL string
	Columns   []string
	Types     []bootstrapType
	Fields    map[string]any
}

// selectBoard fetches boards for the project, prompts the user to pick one,
// and derives the workspace name and slug from the board name.
func selectBoard(ctx context.Context, client API, ui Prompter, projectKey string) (boardSelection, string, string, error) {
	ui.Notify("Bootstrap", fmt.Sprintf("Searching for boards linked to %s...", projectKey))

	boards, err := client.FetchBoardsForProject(ctx, projectKey)
	if err != nil {
		return boardSelection{}, "", "", fmt.Errorf("fetching boards: %w", err)
	}
	if len(boards) == 0 {
		return boardSelection{}, "", "", fmt.Errorf("no boards found for project %s", projectKey)
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
		return boardSelection{}, "", "", err
	}
	if choice < 0 {
		return boardSelection{}, "", "", &core.CancelledError{Operation: "bootstrap"}
	}

	selected := boards[choice]
	sel := boardSelection(selected)

	// Strip superfluous " board" suffix from the Jira board name.
	wsName := selected.Name
	if lower := strings.ToLower(wsName); strings.HasSuffix(lower, " board") {
		wsName = wsName[:len(wsName)-len(" board")]
	}
	wsSlug := strings.ToLower(strings.ReplaceAll(wsName, " ", "_"))
	wsSlug = strings.TrimSuffix(wsSlug, "_board")

	return sel, wsName, wsSlug, nil
}

// discoverWorkspaceContext fetches board config, statuses, JQL, custom fields,
// issue types, and per-type field metadata from the Jira API.
func discoverWorkspaceContext(ctx context.Context, client API, ui Prompter, board boardSelection, projectKey string) (*workspaceDiscovery, error) {
	ui.Notify("Bootstrap", "Fetching board configuration...")
	boardCfg, err := client.FetchBoardConfig(ctx, board.ID)
	if err != nil {
		return nil, fmt.Errorf("fetching board config: %w", err)
	}

	ui.Notify("Bootstrap", "Fetching base JQL filter...")
	filterData, err := client.FetchFilter(ctx, boardCfg.Filter.ID)
	if err != nil {
		return nil, fmt.Errorf("fetching filter: %w", err)
	}

	ui.Notify("Bootstrap", "Fetching status definitions...")
	allStatuses, err := client.FetchStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching statuses: %w", err)
	}
	statusMap := make(map[string]status)
	for _, s := range allStatuses {
		statusMap[s.ID] = s
	}

	var columnNames, visibleStatuses []string
	for _, col := range boardCfg.ColumnConfig.Columns {
		columnNames = append(columnNames, col.Name)
		for _, s := range col.Statuses {
			if st, ok := statusMap[s.ID]; ok {
				visibleStatuses = append(visibleStatuses, st.Name)
			}
		}
	}

	ui.Notify("Bootstrap", "Discovering custom fields...")
	allFields, err := client.FetchFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching fields: %w", err)
	}
	cfMap := discoverCustomFields(allFields)

	ui.Notify("Bootstrap", "Interpolating JQL variables...")
	baseJQL, teamUUID := interpolateBootstrapJQL(filterData.JQL, cfMap)

	ui.Notify("Bootstrap", fmt.Sprintf("Mapping issue types for %s...", projectKey))
	proj, err := client.FetchProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("fetching project: %w", err)
	}
	typesList := buildTypesList(proj.IssueTypes)

	ui.Notify("Bootstrap", "Discovering per-type custom fields...")
	discoverPerTypeFields(ctx, client, projectKey, typesList)

	// Promote fields present on ALL types into workspace-level fields.
	promoted := promoteGlobalFields(typesList, cfMap)
	for alias, fid := range promoted {
		cfMap[alias] = fid
	}

	return &workspaceDiscovery{
		BaseJQL:   baseJQL,
		TeamUUID:  teamUUID,
		StatusJQL: quoteJoin(visibleStatuses),
		Columns:   columnNames,
		Types:     typesList,
		Fields:    cfMap,
	}, nil
}

// writeWorkspaceScaffold assembles the workspace YAML config and writes it
// to out. Pure function — no API calls.
func writeWorkspaceScaffold(out io.Writer, wsName, wsSlug, projectKey, serverURL, serverAlias string, existingCount int, board boardSelection, d *workspaceDiscovery) error {
	wsPayload := map[string]any{
		"server":      serverAlias,
		"name":        wsName,
		"project_key": projectKey,
		"board_id":    board.ID,
		"board_type":  board.Type,
	}
	if d.TeamUUID != "" {
		wsPayload["team_uuid"] = d.TeamUUID
	}
	wsPayload["jql"] = d.BaseJQL
	wsPayload["filters"] = buildBootstrapFilters(board.Type, d.StatusJQL)
	wsPayload["statuses"] = buildStatusesList(d.Columns)
	wsPayload["types"] = d.Types
	wsPayload["fields"] = d.Fields

	scaffold := make(map[string]any)
	scaffold["servers"] = map[string]any{
		serverAlias: map[string]any{
			"provider": core.ProviderJira,
			"url":      serverURL,
		},
	}
	if existingCount == 0 {
		scaffold["default_workspace"] = wsSlug
		scaffold["editor"] = "vim"
	}
	scaffold["workspaces"] = map[string]any{wsSlug: wsPayload}

	yamlBytes, err := yaml.Marshal(scaffold)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}

	header := "# ihj workspace config — generated by bootstrap\n" +
		"#\n" +
		"# Fields are whitelists — only listed fields are displayed in the\n" +
		"# TUI detail view, editor, and exports. Delete any you don't need.\n" +
		"#\n" +
		"# Workspace-level 'fields' contains custom fields present on ALL\n" +
		"# issue types (auto-deduped by bootstrap). Per-type 'fields' holds\n" +
		"# fields unique to that type. Aliases must be unique across both.\n" +
		"#\n" +
		"# Jira often exposes fields with default content on types where\n" +
		"# they aren't relevant so you may want to move things in the workspace\n" +
		"# level to a specific types fields bag.\n" +
		"#\n" +
		"# Format: alias: custom_field_id\n" +
		"#   e.g.  story_points: 10016\n" +
		"#\n"

	if _, err := fmt.Fprint(out, header+string(yamlBytes)); err != nil {
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
	ID          int            `yaml:"id"`
	Name        string         `yaml:"name"`
	Order       int            `yaml:"order"`
	Color       string         `yaml:"color"`
	HasChildren bool           `yaml:"has_children"`
	Fields      map[string]int `yaml:"fields,omitempty"`
}

func buildTypesList(issueTypes []issueType) []bootstrapType {
	known := map[string]struct {
		order int
		color string
	}{
		"initiative": {10, "cyan"}, "epic": {20, "magenta"},
		"story": {30, "blue"}, "task": {30, "default"},
		"bug": {30, "red"}, "sub-task": {40, "white"},
		"subtask": {40, "white"},
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

type bootstrapStatus struct {
	Name  string `yaml:"name"`
	Order int    `yaml:"order"`
	Color string `yaml:"color"`
}

func buildStatusesList(columnNames []string) []bootstrapStatus {
	result := make([]bootstrapStatus, len(columnNames))
	for i, name := range columnNames {
		result[i] = bootstrapStatus{
			Name:  name,
			Order: (i + 1) * 10,
			Color: inferStatusColor(name),
		}
	}
	return result
}

// inferStatusColor maps a status name to a theme color string using
// substring heuristics. The rules mirror terminal.StatusStyle so the
// bootstrap config produces colors consistent with the fallback theme.
func inferStatusColor(name string) string {
	lower := strings.ToLower(name)

	// Ordered most-specific first → least-specific last.
	// Longer / more distinctive substrings must precede short ones
	// that could shadow them (e.g. "review" before "ready").
	switch {
	// Terminal statuses — green.
	case containsAny(lower, "complete", "resolved", "closed", "done"):
		return "green"

	// Blocked / stopped — red.
	case containsAny(lower, "cancel", "block", "stop", "hold"):
		return "red"

	// Review / QA — magenta (before "ready" — "Ready for Review" is magenta).
	case containsAny(lower, "verification", "review", "test", "qa"):
		return "magenta"

	// Active work — blue (before "new" — "Active Development" is blue).
	case containsAny(lower, "progress", "doing", "active", "dev"):
		return "blue"

	// Ready / refined — cyan.
	case containsAny(lower, "approved", "refine", "ready"):
		return "cyan"

	// Backlog / planning — white.
	case containsAny(lower, "backlog", "to do", "todo", "priori", "select", "plan"):
		return "white"

	// Triage / intake — dim (gray). Short matches like "new", "open" last.
	case containsAny(lower, "discovery", "waiting", "pending", "assess",
		"intake", "triage", "new", "open"):
		return "dim"

	default:
		return "default"
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// buildBootstrapFilters generates the filter set based on board type.
// Scrum boards get a sprint-scoped "active" filter; kanban boards get a
// status-based filter with a resolved-date window for recently done items.
func buildBootstrapFilters(boardType, statusJQL string) map[string]string {
	filters := map[string]string{
		"all": "",
		"me":  "assignee = currentUser() AND statusCategory != Done",
	}

	switch boardType {
	case "scrum":
		// Sprint-aware: show items in the active sprint only (exclude
		// future sprints), plus recently resolved items so the user
		// can see what just finished.
		filters["active"] = "sprint IN openSprints() AND sprint NOT IN futureSprints() AND (statusCategory != Done OR resolved >= -2w)"
		// Backlog: items in future sprints or with no sprint assigned.
		filters["backlog"] = "sprint NOT IN openSprints() OR sprint IS EMPTY"
	default:
		// Kanban / simple: no sprint concept. Show items in visible
		// board statuses, plus anything resolved in the last 2 weeks.
		filters["active"] = fmt.Sprintf(
			"status IN (%s) AND (statusCategory != Done OR resolved >= -2w)",
			statusJQL,
		)
	}

	return filters
}

// discoverPerTypeFields fetches createmeta for each type and populates
// the bootstrapType.Fields map with alias → custom field ID entries.
// Only includes custom fields with known plugin types (same filter as
// loadFieldMeta). Fields already captured at workspace level (cfMap) are
// excluded to avoid duplication.
func discoverPerTypeFields(ctx context.Context, client API, projectKey string, types []bootstrapType) {
	for i := range types {
		bt := &types[i]
		typeID := strconv.Itoa(bt.ID)

		metaFields, err := client.FetchCreateMetaFields(ctx, projectKey, typeID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch createmeta for %s (type %s): %v\n", bt.Name, typeID, err)
			continue
		}

		fields := make(map[string]int)
		for _, mf := range metaFields {
			if !strings.HasPrefix(mf.FieldID, "customfield_") {
				continue
			}
			if !isKnownCustomType(mf.Schema.Custom) {
				continue
			}
			idStr := strings.TrimPrefix(mf.FieldID, "customfield_")
			fid, err := strconv.Atoi(idStr)
			if err != nil {
				continue
			}

			alias := nameToKey(mf.Name)
			if alias == "" {
				alias = idStr
			}
			// Avoid key collisions within the same type.
			if _, exists := fields[alias]; exists {
				alias = alias + "_" + idStr
			}
			fields[alias] = fid
		}

		if len(fields) > 0 {
			bt.Fields = fields
		}
	}
}

// promoteGlobalFields identifies custom fields present on every type and
// moves them from per-type Fields into a shared map. Fields already in cfMap
// (well-known globals like team, epic_name) are skipped. Returns the promoted
// alias→ID entries; the caller merges them into cfMap.
func promoteGlobalFields(types []bootstrapType, cfMap map[string]any) map[string]int {
	if len(types) == 0 {
		return nil
	}

	// Build a set of field IDs already in cfMap so we don't double-add.
	existing := make(map[int]bool)
	for _, v := range cfMap {
		if fid, ok := v.(int); ok {
			existing[fid] = true
		}
	}

	// Count how many types each field ID appears on.
	idCount := make(map[int]int)
	// Track the alias used for each field ID (first-type-wins).
	idAlias := make(map[int]string)
	for _, bt := range types {
		for alias, fid := range bt.Fields {
			idCount[fid]++
			if _, seen := idAlias[fid]; !seen {
				idAlias[fid] = alias
			}
		}
	}

	// Promote fields present on ALL types.
	promoted := make(map[string]int)
	total := len(types)
	for fid, count := range idCount {
		if count == total && !existing[fid] {
			promoted[idAlias[fid]] = fid
		}
	}

	// Remove promoted and already-global fields from per-type maps.
	removeIDs := make(map[int]bool, len(promoted)+len(existing))
	for _, fid := range promoted {
		removeIDs[fid] = true
	}
	for fid := range existing {
		removeIDs[fid] = true
	}
	for i := range types {
		for alias, fid := range types[i].Fields {
			if removeIDs[fid] {
				delete(types[i].Fields, alias)
			}
		}
		if len(types[i].Fields) == 0 {
			types[i].Fields = nil
		}
	}

	return promoted
}

func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf(`"%s"`, s)
	}
	return strings.Join(quoted, ", ")
}
