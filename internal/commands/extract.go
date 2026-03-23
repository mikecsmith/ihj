package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
)

// --- Extract scope constants ---

const (
	ScopeSelectedOnly = "Selected issue only"
	ScopeWithChildren = "Selected + children"
	ScopeWithParent   = "Selected + parent"
	ScopeFullFamily   = "Full family (parent + siblings + children)"
	ScopeEntireBoard  = "Entire board"
)

// ScopeOptions returns the available scope options for the given issue.
func ScopeOptions(hasParent bool) []string {
	opts := []string{ScopeSelectedOnly, ScopeWithChildren}
	if hasParent {
		opts = append(opts, ScopeWithParent, ScopeFullFamily)
	}
	opts = append(opts, ScopeEntireBoard)
	return opts
}

// CollectExtractKeys determines which issue keys to include based on scope,
// working from the IssueView registry. Used by both CLI and TUI.
func CollectExtractKeys(issueKey, scope string, registry map[string]*jira.IssueView) map[string]bool {
	keys := map[string]bool{issueKey: true}
	target := registry[issueKey]
	if target == nil {
		return keys
	}

	switch scope {
	case ScopeSelectedOnly:
		// Just the target.

	case ScopeWithChildren:
		for k := range target.Children {
			keys[k] = true
		}

	case ScopeWithParent:
		if target.ParentKey != "" {
			keys[target.ParentKey] = true
		}

	case ScopeFullFamily:
		// Children of target.
		for k := range target.Children {
			keys[k] = true
		}
		// Parent.
		if target.ParentKey != "" {
			keys[target.ParentKey] = true
			// Siblings = other children of the same parent.
			if parent, ok := registry[target.ParentKey]; ok {
				for k := range parent.Children {
					keys[k] = true
					// Also include nieces/nephews (children of siblings).
					if sibling, ok := registry[k]; ok {
						for ck := range sibling.Children {
							keys[ck] = true
						}
					}
				}
			}
		}

	case ScopeEntireBoard:
		for k := range registry {
			keys[k] = true
		}
	}

	return keys
}

// BuildExtractXML produces the XML context for an LLM prompt from IssueView data.
// Used by both CLI and TUI extract flows.
func BuildExtractXML(prompt string, keys map[string]bool, registry map[string]*jira.IssueView, board *config.BoardConfig) string {
	var b strings.Builder
	b.WriteString("<context>\n  <instruction>\n    ")
	b.WriteString(prompt)
	b.WriteString("\n  </instruction>\n")

	if len(keys) == 1 {
		b.WriteString("  <output_format>\n")
		b.WriteString("    Output as a single ihj-compatible Markdown block with YAML frontmatter.\n")
		b.WriteString("    Wrap the entire response in ```markdown code fences.\n")
		b.WriteString("  </output_format>\n")
	} else {
		schema := config.HierarchySchema(board)
		schemaJSON, _ := json.MarshalIndent(schema, "    ", "  ")
		b.WriteString("  <output_format>\n")
		b.WriteString("    Output as structured YAML validating against this schema:\n")
		b.WriteString("    <json_schema>\n    ")
		b.Write(schemaJSON)
		b.WriteString("\n    </json_schema>\n  </output_format>\n")
	}

	b.WriteString("  <issues>\n")
	typesUsed := make(map[string]bool)

	for key := range keys {
		iv, ok := registry[key]
		if !ok {
			continue
		}
		typesUsed[iv.Type] = true

		descMD := ""
		if iv.Desc != nil {
			descMD = strings.TrimSpace(document.RenderMarkdown(iv.Desc))
		}

		fmt.Fprintf(&b, "    <issue key=%q type=%q status=%q", key, iv.Type, iv.Status)
		if iv.ParentKey != "" {
			fmt.Fprintf(&b, " parent=%q", iv.ParentKey)
		}
		b.WriteString(">\n")
		fmt.Fprintf(&b, "      <summary>%s</summary>\n", iv.Summary)
		if descMD != "" {
			fmt.Fprintf(&b, "      <description>\n%s\n      </description>\n", descMD)
		}
		b.WriteString("    </issue>\n")
	}
	b.WriteString("  </issues>\n")

	// Include issue type templates if available.
	hasTemplates := false
	for _, t := range board.Types {
		if typesUsed[t.Name] && t.Template != "" {
			if !hasTemplates {
				b.WriteString("  <templates>\n")
				hasTemplates = true
			}
			fmt.Fprintf(&b, "    <template type=%q>\n%s\n    </template>\n",
				t.Name, strings.TrimSpace(t.Template))
		}
	}
	if hasTemplates {
		b.WriteString("  </templates>\n")
	}

	b.WriteString("</context>")
	return b.String()
}

// --- CLI Extract command ---

func Extract(app *App, issueKey string) error {
	prefix := strings.ToUpper(strings.SplitN(issueKey, "-", 2)[0])
	var boardSlug string
	var boardCfg *config.BoardConfig
	for slug, b := range app.Config.Boards {
		if strings.EqualFold(b.ProjectKey, prefix) {
			boardSlug = slug
			boardCfg = b
			break
		}
	}
	if boardCfg == nil {
		return fmt.Errorf("could not map %s to a known board", issueKey)
	}

	// Build IssueView registry from cached data.
	issueMap := loadCachedIssueMap(app.CacheDir, boardSlug)
	issues := make([]client.Issue, 0, len(issueMap))
	for _, iss := range issueMap {
		issues = append(issues, iss)
	}
	registry := jira.BuildRegistry(issues)
	jira.LinkChildren(registry)

	target := registry[issueKey]
	if target == nil {
		return fmt.Errorf("issue %s not found in local cache", issueKey)
	}

	options := ScopeOptions(target.ParentKey != "")

	choice, err := app.UI.Select(fmt.Sprintf("LLM Extract: %s", issueKey), options)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &CancelledError{Operation: "extract"}
	}

	collected := CollectExtractKeys(issueKey, options[choice], registry)

	delimiter := "_END_OF_PROMPT_"
	boilerplate := fmt.Sprintf("\n\n%s\nType your LLM prompt above. XML context will append automatically.\n", delimiter)

	raw, err := app.UI.EditText(boilerplate, "llm_prompt_", 1, "")
	if err != nil {
		return fmt.Errorf("opening editor: %w", err)
	}

	prompt := strings.TrimSpace(strings.SplitN(raw, delimiter, 2)[0])
	if prompt == "" {
		return &CancelledError{Operation: "extract"}
	}

	xml := BuildExtractXML(prompt, collected, registry, boardCfg)

	if err := app.UI.CopyToClipboard(xml); err != nil {
		return fmt.Errorf("copying to clipboard: %w", err)
	}

	app.UI.Notify("LLM Ready", fmt.Sprintf("Copied XML context (%d issues) to clipboard!", len(collected)))
	return nil
}

// --- Cache helpers ---

func loadCachedIssueMap(cacheDir, boardSlug string) map[string]client.Issue {
	files, _ := filepath.Glob(filepath.Join(cacheDir, boardSlug+"_*.json"))
	result := make(map[string]client.Issue)
	for _, f := range files {
		if strings.HasSuffix(f, ".state.json") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var issues []client.Issue
		if json.Unmarshal(data, &issues) == nil {
			for _, iss := range issues {
				result[iss.Key] = iss
			}
		}
	}
	return result
}
