package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mikecsmith/ihj/internal/commands"
	"github.com/mikecsmith/ihj/internal/core"
)

// startExtract initiates the two-step extract workflow: scope selection → LLM prompt.
func (m *AppModel) startExtract(iss *core.WorkItem) {
	scopes := commands.ScopeOptions(iss.ParentID != "")
	m.extractIssueKey = iss.ID
	m.extractScopes = scopes
	m.popup.ShowSelect("extract-scope", "Extract Scope: "+iss.ID, scopes)
}

// handleExtractResult processes popup results for the extract workflow.
// Returns the updated model, an optional command, and whether the result was handled.
func (m AppModel) handleExtractResult(result *PopupResult) (tea.Model, tea.Cmd, bool) {
	switch result.ID {
	case "extract-scope":
		if result.Index >= 0 && result.Index < len(m.extractScopes) {
			m.extractScopeIdx = result.Index
			m.popup.ShowInput("extract-prompt", "LLM Prompt: "+m.extractIssueKey, "Describe what you want the LLM to do...")
			return m, nil, true
		}

	case "extract-prompt":
		if result.Text != "" {
			prompt := result.Text
			issueKey := m.extractIssueKey
			scopeName := m.extractScopes[m.extractScopeIdx]
			registry := m.registry
			board := m.ws
			return m, m.async(func() (string, error) {
				keys := commands.CollectExtractKeys(issueKey, scopeName, registry)
				xml := commands.BuildExtractXML(prompt, keys, registry, board)
				if err := m.runtime.UI.CopyToClipboard(xml); err != nil {
					return "", err
				}
				return fmt.Sprintf("LLM context copied (%d issues)", len(keys)), nil
			}), true
		}
	}

	return m, nil, false
}
