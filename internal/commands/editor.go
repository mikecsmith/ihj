package commands

import (
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// CalculateCursor returns the editor cursor line and search pattern
// for the frontmatter document. If summary is empty, the cursor targets
// the summary field; otherwise it positions after the closing ---.
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

// TypeNames returns the display names of all configured types.
func TypeNames(ws *core.Workspace) []string {
	names := make([]string, len(ws.Types))
	for i, t := range ws.Types {
		names[i] = t.Name
	}
	return names
}

// offerRecovery presents the user with recovery options after an error.
func offerRecovery(s *Session, contents, errMsg string) (string, error) {
	s.UI.Notify("Error", errMsg)

	choice, err := s.UI.Select("What now?", []string{
		"Re-edit",
		"Copy to clipboard and abort",
		"Abort",
	})
	if err != nil {
		return "", err
	}

	switch choice {
	case 0:
		return s.UI.EditText(contents, "ihj_", 0, "")
	case 1:
		if clipErr := s.UI.CopyToClipboard(contents); clipErr != nil {
			s.UI.Notify("Warning", "Could not copy to clipboard")
		} else {
			s.UI.Notify("Rescue", "Buffer copied to clipboard.")
		}
		return "", nil
	default:
		return "", nil
	}
}

// first returns the first non-empty string from the arguments.
func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// workItemToMetadata converts a WorkItem to the frontmatter metadata map
// used by the editor.
func workItemToMetadata(item *core.WorkItem) map[string]string {
	m := map[string]string{
		"key":     item.ID,
		"type":    item.Type,
		"status":  item.Status,
		"summary": item.Summary,
	}
	if item.ParentID != "" {
		m["parent"] = item.ParentID
	}
	if v := item.StringField("priority"); v != "" {
		m["priority"] = v
	}
	return m
}

// frontmatterToWorkItem builds a WorkItem from parsed frontmatter and
// a description AST. Used by the create flow.
func frontmatterToWorkItem(fm map[string]string, description *document.Node) *core.WorkItem {
	item := &core.WorkItem{
		Summary: fm["summary"],
		Type:    fm["type"],
		Status:  fm["status"],
	}
	if fm["parent"] != "" {
		item.ParentID = fm["parent"]
	}
	if description != nil {
		item.Description = description
	}
	fields := make(map[string]any)
	if fm["priority"] != "" {
		fields["priority"] = fm["priority"]
	}
	if strings.EqualFold(fm["sprint"], "true") {
		fields["sprint"] = true
	}
	if len(fields) > 0 {
		item.Fields = fields
	}
	return item
}

// frontmatterToChanges builds a Changes struct from edited frontmatter,
// comparing against the original status for transition detection.
func frontmatterToChanges(fm map[string]string, description *document.Node, origItem *core.WorkItem) *core.Changes {
	changes := &core.Changes{}
	hasChange := false

	if fm["summary"] != origItem.Summary {
		changes.Summary = strPtr(fm["summary"])
		hasChange = true
	}
	if !strings.EqualFold(fm["type"], origItem.Type) {
		changes.Type = strPtr(fm["type"])
		hasChange = true
	}
	if !strings.EqualFold(fm["status"], origItem.Status) {
		changes.Status = strPtr(fm["status"])
		hasChange = true
	}
	if fm["parent"] != origItem.ParentID {
		changes.ParentID = strPtr(fm["parent"])
		hasChange = true
	}
	if description != nil {
		newMD := strings.TrimSpace(document.RenderMarkdown(description))
		origMD := origItem.DescriptionMarkdown()
		if newMD != origMD {
			changes.Description = description
			hasChange = true
		}
	}

	fields := make(map[string]any)
	if fm["priority"] != "" && fm["priority"] != origItem.StringField("priority") {
		fields["priority"] = fm["priority"]
	}
	if strings.EqualFold(fm["sprint"], "true") {
		fields["sprint"] = true
	}
	if len(fields) > 0 {
		changes.Fields = fields
		hasChange = true
	}

	if !hasChange {
		return nil
	}
	return changes
}

func strPtr(s string) *string {
	return &s
}
