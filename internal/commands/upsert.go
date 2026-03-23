package commands

import (
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
)

type UpsertOpts struct {
	Board     string
	IssueKey  string
	IsEdit    bool
	Overrides map[string]string
}

func Upsert(app *App, opts UpsertOpts) error {
	board, err := app.Config.ResolveBoard(opts.Board)
	if err != nil {
		return err
	}

	schemaDict := config.FrontmatterSchema(app.Config, board)
	schemaPath, err := config.WriteFrontmatterSchema(app.CacheDir, board.Slug, schemaDict)
	if err != nil {
		return fmt.Errorf("writing schema: %w", err)
	}

	metadata := make(map[string]string)
	bodyText := ""
	origStatus := ""

	if opts.IsEdit {
		if opts.IssueKey == "" {
			return fmt.Errorf("issue key is required for edit")
		}
		if err := populateEditMetadata(app, opts, metadata, &bodyText, &origStatus); err != nil {
			return err
		}
	} else {
		if err := populateCreateMetadata(app, board, opts, metadata, &bodyText, &origStatus); err != nil {
			return err
		}
	}

	initialDoc := config.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
	cursorLine, searchPat := CalculateCursor(initialDoc, metadata["summary"])

	edited, err := app.UI.EditText(initialDoc, "jira_", cursorLine, searchPat)
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	if strings.TrimSpace(edited) == strings.TrimSpace(initialDoc) {
		return &CancelledError{Operation: "upsert"}
	}

	targetKey := opts.IssueKey

	for {
		fm, mdBody, parseErr := config.ParseFrontmatter(edited)
		if parseErr != nil {
			retry, err := offerRecovery(app, edited, fmt.Sprintf("YAML error: %v", parseErr))
			if err != nil || retry == "" {
				return &CancelledError{Operation: "upsert"}
			}
			edited = retry
			continue
		}

		if errMsg := validateFrontmatter(fm); errMsg != "" {
			retry, err := offerRecovery(app, edited, errMsg)
			if err != nil || retry == "" {
				return &CancelledError{Operation: "upsert"}
			}
			edited = retry
			continue
		}

		ast, err := document.ParseMarkdownString(mdBody)
		if err != nil {
			return fmt.Errorf("parsing description: %w", err)
		}
		adfBody := document.RenderADFValue(ast)

		payload := jira.BuildUpsertPayload(
			fm, adfBody, board.Types, app.Config.CustomFields,
			board.ProjectKey, board.TeamUUID,
		)

		if opts.IsEdit {
			if err := app.Client.UpdateIssue(opts.IssueKey, payload); err != nil {
				retry, retryErr := offerRecovery(app, edited, fmt.Sprintf("API rejected update: %v", err))
				if retryErr != nil || retry == "" {
					return err
				}
				edited = retry
				continue
			}
			app.UI.Notify("Jira Success", fmt.Sprintf("Updated %s", targetKey))
		} else {
			created, err := app.Client.CreateIssue(payload)
			if err != nil {
				retry, retryErr := offerRecovery(app, edited, fmt.Sprintf("API rejected create: %v", err))
				if retryErr != nil || retry == "" {
					return err
				}
				edited = retry
				continue
			}
			targetKey = created.Key
			app.UI.Notify("Jira Success", fmt.Sprintf("Created %s", targetKey))
		}

		// Post-upsert actions with already-parsed frontmatter (no re-parse).
		postUpsert(app, board, fm, targetKey, origStatus)
		return nil
	}
}

func populateEditMetadata(app *App, opts UpsertOpts, metadata map[string]string, body, origStatus *string) error {
	issues, err := jira.FetchAllIssues(app.Client, fmt.Sprintf("key = %s", opts.IssueKey), app.Config.FormattedCustomFields)
	if err != nil {
		return fmt.Errorf("fetching issue: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("issue %s not found", opts.IssueKey)
	}

	f := &issues[0].Fields
	*origStatus = f.Status.Name

	metadata["key"] = opts.IssueKey
	metadata["type"] = first(opts.Overrides["type"], f.IssueType.Name)
	metadata["priority"] = first(opts.Overrides["priority"], f.Priority.Name, "Medium")
	metadata["status"] = first(opts.Overrides["status"], *origStatus)
	metadata["summary"] = first(opts.Overrides["summary"], f.Summary)

	if f.Parent != nil {
		metadata["parent"] = first(opts.Overrides["parent"], f.Parent.Key)
	} else {
		metadata["parent"] = opts.Overrides["parent"]
	}

	for cfName, cfID := range app.Config.CustomFields {
		if cfName == "team" {
			metadata["team"] = first(opts.Overrides["team"], "true")
		} else {
			fieldID := fmt.Sprintf("customfield_%d", cfID)
			if val := f.CustomString(fieldID); val != "" {
				metadata[cfName] = val
			}
		}
	}

	if len(f.Description) > 0 && string(f.Description) != "null" {
		if ast, err := document.ParseADF(f.Description); err == nil {
			*body = strings.TrimSpace(document.RenderMarkdown(ast))
		}
	}
	return nil
}

func populateCreateMetadata(app *App, board *config.BoardConfig, opts UpsertOpts, metadata map[string]string, body, origStatus *string) error {
	typeNames := make([]string, len(board.Types))
	for i, t := range board.Types {
		typeNames[i] = t.Name
	}

	selectedType := opts.Overrides["type"]
	if selectedType == "" {
		choice, err := app.UI.Select("Create New Issue", typeNames)
		if err != nil {
			return err
		}
		if choice < 0 {
			return &CancelledError{Operation: "create"}
		}
		selectedType = typeNames[choice]
	}

	*origStatus = "Backlog"
	metadata["type"] = selectedType
	metadata["priority"] = first(opts.Overrides["priority"], "Medium")
	metadata["status"] = first(opts.Overrides["status"], *origStatus)
	metadata["parent"] = opts.Overrides["parent"]
	metadata["summary"] = opts.Overrides["summary"]

	if _, ok := app.Config.CustomFields["team"]; ok {
		metadata["team"] = first(opts.Overrides["team"], "true")
	}

	for _, t := range board.Types {
		if t.Name == selectedType && t.Template != "" {
			*body = strings.TrimSpace(t.Template)
			break
		}
	}
	return nil
}

func validateFrontmatter(fm map[string]string) string {
	if fm["summary"] == "" {
		return "Summary is required."
	}
	if strings.EqualFold(fm["type"], "sub-task") && fm["parent"] == "" {
		return "Sub-tasks require a parent issue key."
	}
	return ""
}

func offerRecovery(app *App, contents, errMsg string) (string, error) {
	app.UI.Notify("Error", errMsg)

	choice, err := app.UI.Select("What now?", []string{
		"Re-edit",
		"Copy to clipboard and abort",
		"Abort",
	})
	if err != nil {
		return "", err
	}

	switch choice {
	case 0:
		return app.UI.EditText(contents, "jira_", 0, "")
	case 1:
		if clipErr := app.UI.CopyToClipboard(contents); clipErr != nil {
			app.UI.Notify("Warning", "Could not copy to clipboard")
		} else {
			app.UI.Notify("Rescue", "Buffer copied to clipboard.")
		}
		return "", nil
	default:
		return "", nil
	}
}

// postUpsert handles sprint assignment and status transition after a
// successful create/update. Takes the already-parsed frontmatter map
// to avoid re-parsing and the silent failure path.
func postUpsert(app *App, board *config.BoardConfig, fm map[string]string, targetKey, origStatus string) {
	if strings.EqualFold(fm["sprint"], "true") {
		ok, err := jira.AssignToSprint(app.Client, board.ID, targetKey)
		if err != nil {
			app.UI.Notify("Sprint Error", fmt.Sprintf("Failed to add to sprint: %v", err))
		} else if ok {
			app.UI.Notify("Sprint", fmt.Sprintf("Added %s to active sprint.", targetKey))
		} else {
			app.UI.Notify("Sprint", "No active sprint found.")
		}
	}

	if newStatus := fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, origStatus) {
		if err := jira.PerformTransition(app.Client, targetKey, newStatus); err != nil {
			app.UI.Notify("Warning", fmt.Sprintf("Could not transition to '%s': %v", newStatus, err))
		} else {
			app.UI.Notify(targetKey, fmt.Sprintf("Moved to %s", newStatus))
		}
	}
}

func CalculateCursor(doc, summary string) (int, string) {
	if summary == "" {
		return 0, "^summary:"
	}
	dashes := 0
	for i, line := range strings.Split(doc, "\n") {
		if strings.TrimSpace(line) == "---" {
			dashes++
			if dashes == 2 {
				return i + 2, ""
			}
		}
	}
	return 0, ""
}

func TypeNames(board *config.BoardConfig) []string {
	names := make([]string, len(board.Types))
	for i, t := range board.Types {
		names[i] = t.Name
	}
	return names
}

// --- Exported helpers for TUI-driven upsert flow ---
// The TUI calls these individually to split the upsert pipeline into
// discrete phases orchestrated by the Bubble Tea message loop.
// The CLI path continues to use Upsert() unchanged.

// PrepareUpsert resolves the board, generates the JSON schema, populates
// metadata (edit mode only), and builds the frontmatter document.
// For create mode, call PrepareCreateMetadata separately after type selection.
func PrepareUpsert(app *App, opts UpsertOpts) (
	board *config.BoardConfig, schemaPath string,
	metadata map[string]string, bodyText, origStatus, initialDoc string,
	cursorLine int, searchPat string, err error,
) {
	board, err = app.Config.ResolveBoard(opts.Board)
	if err != nil {
		return
	}

	schemaDict := config.FrontmatterSchema(app.Config, board)
	schemaPath, err = config.WriteFrontmatterSchema(app.CacheDir, board.Slug, schemaDict)
	if err != nil {
		err = fmt.Errorf("writing schema: %w", err)
		return
	}

	metadata = make(map[string]string)

	if opts.IsEdit {
		if opts.IssueKey == "" {
			err = fmt.Errorf("issue key is required for edit")
			return
		}
		if err = populateEditMetadata(app, opts, metadata, &bodyText, &origStatus); err != nil {
			return
		}
	}
	// Create mode metadata is handled by PrepareCreateMetadata after
	// the TUI popup selects the issue type.

	initialDoc = config.BuildFrontmatterDoc(schemaPath, metadata, bodyText)
	cursorLine, searchPat = CalculateCursor(initialDoc, metadata["summary"])
	return
}

// PrepareCreateMetadata populates metadata for create mode once the issue
// type is known. Called after TUI popup type selection.
func PrepareCreateMetadata(app *App, board *config.BoardConfig, opts UpsertOpts, selectedType string) (
	metadata map[string]string, bodyText, origStatus string,
) {
	metadata = make(map[string]string)
	origStatus = "Backlog"
	metadata["type"] = selectedType
	metadata["priority"] = first(opts.Overrides["priority"], "Medium")
	metadata["status"] = first(opts.Overrides["status"], origStatus)
	metadata["parent"] = opts.Overrides["parent"]
	metadata["summary"] = opts.Overrides["summary"]

	if _, ok := app.Config.CustomFields["team"]; ok {
		metadata["team"] = first(opts.Overrides["team"], "true")
	}

	for _, t := range board.Types {
		if t.Name == selectedType && t.Template != "" {
			bodyText = strings.TrimSpace(t.Template)
			break
		}
	}
	return
}

// SubmitUpsert parses, validates, and submits the edited document.
// On success, returns issueKey and the parsed frontmatter map.
// If recoverableMsg is non-empty, the error is recoverable (user can re-edit).
func SubmitUpsert(app *App, board *config.BoardConfig, opts UpsertOpts, edited string) (
	issueKey string, fm map[string]string, err error, recoverableMsg string,
) {
	fm, mdBody, parseErr := config.ParseFrontmatter(edited)
	if parseErr != nil {
		recoverableMsg = fmt.Sprintf("YAML error: %v", parseErr)
		return
	}

	if errMsg := validateFrontmatter(fm); errMsg != "" {
		recoverableMsg = errMsg
		return
	}

	ast, astErr := document.ParseMarkdownString(mdBody)
	if astErr != nil {
		err = fmt.Errorf("parsing description: %w", astErr)
		return
	}
	adfBody := document.RenderADFValue(ast)

	payload := jira.BuildUpsertPayload(
		fm, adfBody, board.Types, app.Config.CustomFields,
		board.ProjectKey, board.TeamUUID,
	)

	if opts.IsEdit {
		if err = app.Client.UpdateIssue(opts.IssueKey, payload); err != nil {
			recoverableMsg = fmt.Sprintf("API rejected update: %v", err)
			return
		}
		issueKey = opts.IssueKey
	} else {
		created, createErr := app.Client.CreateIssue(payload)
		if createErr != nil {
			err = createErr
			recoverableMsg = fmt.Sprintf("API rejected create: %v", err)
			return
		}
		issueKey = created.Key
	}
	return
}

// PostUpsertNotifications runs sprint assignment and status transition,
// returning notification strings instead of calling Notify() directly.
func PostUpsertNotifications(app *App, board *config.BoardConfig, fm map[string]string, targetKey, origStatus string) []string {
	var notes []string

	if strings.EqualFold(fm["sprint"], "true") {
		ok, err := jira.AssignToSprint(app.Client, board.ID, targetKey)
		if err != nil {
			notes = append(notes, fmt.Sprintf("Sprint Error: %v", err))
		} else if ok {
			notes = append(notes, fmt.Sprintf("Added %s to active sprint", targetKey))
		} else {
			notes = append(notes, "No active sprint found")
		}
	}

	if newStatus := fm["status"]; newStatus != "" && !strings.EqualFold(newStatus, origStatus) {
		if err := jira.PerformTransition(app.Client, targetKey, newStatus); err != nil {
			notes = append(notes, fmt.Sprintf("⚠ Could not transition to '%s': %v", newStatus, err))
		} else {
			notes = append(notes, fmt.Sprintf("%s → %s", targetKey, newStatus))
		}
	}

	return notes
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
