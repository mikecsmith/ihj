package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
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
// working from the WorkItem registry. Used by both CLI and TUI.
func CollectExtractKeys(issueKey, scope string, registry map[string]*core.WorkItem) map[string]bool {
	keys := map[string]bool{issueKey: true}
	target := registry[issueKey]
	if target == nil {
		return keys
	}

	switch scope {
	case ScopeSelectedOnly:
		// Just the target.

	case ScopeWithChildren:
		for _, child := range target.Children {
			keys[child.ID] = true
		}

	case ScopeWithParent:
		if target.ParentID != "" {
			keys[target.ParentID] = true
		}

	case ScopeFullFamily:
		// Children of target.
		for _, child := range target.Children {
			keys[child.ID] = true
		}
		// Parent.
		if target.ParentID != "" {
			keys[target.ParentID] = true
			// Siblings = other children of the same parent.
			if parent, ok := registry[target.ParentID]; ok {
				for _, child := range parent.Children {
					keys[child.ID] = true
					// Also include nieces/nephews (children of siblings).
					if sibling, ok := registry[child.ID]; ok {
						for _, nephew := range sibling.Children {
							keys[nephew.ID] = true
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

// BuildExtractXML produces the XML context for an LLM prompt from WorkItem data.
// Used by both CLI and TUI extract flows.
func BuildExtractXML(prompt string, keys map[string]bool, registry map[string]*core.WorkItem, ws *core.Workspace) string {
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
		schema := core.ManifestSchema(ws)
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
		if iv.Description != nil {
			descMD = strings.TrimSpace(document.RenderMarkdown(iv.Description))
		}

		fmt.Fprintf(&b, "    <issue key=%q type=%q status=%q", key, iv.Type, iv.Status)
		if iv.ParentID != "" {
			fmt.Fprintf(&b, " parent=%q", iv.ParentID)
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
	for _, t := range ws.Types {
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

func Extract(s *Session, workspaceSlug, issueKey string) error {
	ws, err := s.Config.ResolveWorkspace(workspaceSlug)
	if err != nil {
		return err
	}

	// Fetch items via Provider and build registry.
	items, err := s.Provider.Search(context.TODO(), "active", nil)
	if err != nil {
		return fmt.Errorf("fetching workspace data: %w", err)
	}

	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)

	target := registry[issueKey]
	if target == nil {
		return fmt.Errorf("issue %s not found", issueKey)
	}

	options := ScopeOptions(target.ParentID != "")

	choice, err := s.UI.Select(fmt.Sprintf("LLM Extract: %s", issueKey), options)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &CancelledError{Operation: "extract"}
	}

	collected := CollectExtractKeys(issueKey, options[choice], registry)

	delimiter := "_END_OF_PROMPT_"
	boilerplate := fmt.Sprintf("\n\n%s\nType your LLM prompt above. XML context will append automatically.\n", delimiter)

	raw, err := s.UI.EditText(boilerplate, "llm_prompt_", 1, "")
	if err != nil {
		return fmt.Errorf("opening editor: %w", err)
	}

	prompt := strings.TrimSpace(strings.SplitN(raw, delimiter, 2)[0])
	if prompt == "" {
		return &CancelledError{Operation: "extract"}
	}

	xml := BuildExtractXML(prompt, collected, registry, ws)

	if err := s.UI.CopyToClipboard(xml); err != nil {
		return fmt.Errorf("copying to clipboard: %w", err)
	}

	s.UI.Notify("LLM Ready", fmt.Sprintf("Copied XML context (%d issues) to clipboard!", len(collected)))
	return nil
}
