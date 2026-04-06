package tui

import "time"

func (m *AppModel) setNotify(msg string) {
	m.notify = msg
	m.notifyAt = time.Now()
	m.ui.Emit(EventNotify, "message", msg)
}

// resolveWorkspaceSlug finds the workspace slug for a display label.
// Labels may include a server alias suffix like "My Team (prod-jira)".
func (m *AppModel) resolveWorkspaceSlug(label string) string {
	for slug, ws := range m.runtime.Workspaces {
		name := ws.Name
		if name == "" {
			name = slug
		}
		candidate := name
		if ws.ServerAlias != "" {
			candidate += " (" + ws.ServerAlias + ")"
		}
		if candidate == label {
			return slug
		}
	}
	return label // Fallback: treat label as slug.
}
