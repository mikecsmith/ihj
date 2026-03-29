package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// ExtractOptions controls the extract command behaviour. Empty fields
// fall through to interactive selection.
type ExtractOptions struct {
	Scope  string // Short scope name: "selected", "children", "parent", "family", "workspace". Empty = interactive.
	Prompt string // Inline prompt text. Empty = open editor.
	Copy   bool   // If true, copy to clipboard instead of writing to stdout.
}

// scopeShortNames maps CLI-friendly short names to the internal scope constants.
var scopeShortNames = map[string]string{
	"selected":  ScopeSelectedOnly,
	"children":  ScopeWithChildren,
	"parent":    ScopeWithParent,
	"family":    ScopeFullFamily,
	"workspace": ScopeEntireWorkspace,
}

// ResolveScopeName maps a short CLI scope name to the internal scope constant.
func ResolveScopeName(short string) (string, error) {
	if v, ok := scopeShortNames[short]; ok {
		return v, nil
	}
	valid := make([]string, 0, len(scopeShortNames))
	for k := range scopeShortNames {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	return "", fmt.Errorf("invalid scope %q, valid values: %s", short, strings.Join(valid, ", "))
}

const (
	ScopeSelectedOnly    = "Selected issue only"
	ScopeWithChildren    = "Selected + children"
	ScopeWithParent      = "Selected + parent"
	ScopeFullFamily      = "Full family (parent + siblings + children)"
	ScopeEntireWorkspace = "Entire workspace"
)

// ScopeOptions returns the available scope options for the given issue.
func ScopeOptions(hasParent bool) []string {
	opts := []string{ScopeSelectedOnly, ScopeWithChildren}
	if hasParent {
		opts = append(opts, ScopeWithParent, ScopeFullFamily)
	}
	opts = append(opts, ScopeEntireWorkspace)
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

	case ScopeEntireWorkspace:
		for k := range registry {
			keys[k] = true
		}
	}

	return keys
}

// xmlEscape returns s with XML special characters escaped.
func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// sortedKeys returns the keys of a map[string]bool in sorted order.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// BuildExtractXML produces the XML context for an LLM prompt from WorkItem data.
// Used by both CLI and TUI extract flows. Field defs control which fields are
// included in the issue elements.
func BuildExtractXML(prompt string, keys map[string]bool, registry map[string]*core.WorkItem, ws *core.Workspace, defs []core.FieldDef) string {
	var b strings.Builder
	b.WriteString("<context>\n  <instruction>\n    ")
	b.WriteString(xmlEscape(prompt))
	b.WriteString("\n  </instruction>\n")

	b.WriteString("  <guidance>\n")
	b.WriteString("    - This is an interactive conversation. Ask clarifying questions before producing output.\n")
	b.WriteString("    - Ask the user if they have supporting materials to share — meeting transcripts, discovery documents, proposals, specs, or design docs can dramatically improve output quality.\n")
	b.WriteString("    - Once you understand the scope, produce a brief plan and wait for confirmation before generating the structured YAML output.\n")
	b.WriteString("    - Preserve all existing issue keys exactly as provided.\n")
	b.WriteString("    - Do not invent new issue keys — if new issues are needed, omit the key field.\n")
	b.WriteString("  </guidance>\n")

	if len(keys) == 1 {
		b.WriteString("  <output_format>\n")
		b.WriteString("    Output as a single ihj-compatible Markdown block with YAML frontmatter.\n")
		b.WriteString("    Wrap the entire response in ```markdown code fences.\n")
		b.WriteString("  </output_format>\n")
	} else {
		schema := core.ManifestSchema(ws, defs)
		schemaJSON, _ := json.MarshalIndent(schema, "    ", "  ")
		b.WriteString("  <output_format>\n")
		b.WriteString("    Output as structured YAML validating against this schema:\n")
		b.WriteString("    <json_schema>\n    ")
		b.Write(schemaJSON)
		b.WriteString("\n    </json_schema>\n  </output_format>\n")
	}

	b.WriteString("  <issues>\n")
	typesUsed := make(map[string]bool)

	for _, key := range sortedKeys(keys) {
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
		fmt.Fprintf(&b, "      <summary>%s</summary>\n", xmlEscape(iv.Summary))

		for _, def := range defs {
			if def.Visibility == core.FieldReadOnly {
				continue
			}
			val, ok := iv.Fields[def.Key]
			if !ok || core.IsZeroFieldValue(val) {
				continue
			}
			switch v := val.(type) {
			case []string:
				fmt.Fprintf(&b, "      <%s>%s</%s>\n", def.Key, xmlEscape(strings.Join(v, ", ")), def.Key)
			default:
				fmt.Fprintf(&b, "      <%s>%s</%s>\n", def.Key, xmlEscape(fmt.Sprintf("%v", v)), def.Key)
			}
		}

		if descMD != "" {
			fmt.Fprintf(&b, "      <description><![CDATA[\n%s\n      ]]></description>\n", descMD)
		}
		b.WriteString("    </issue>\n")
	}
	b.WriteString("  </issues>\n")

	hasTemplates := false
	for _, t := range ws.Types {
		if typesUsed[t.Name] && t.Template != "" {
			if !hasTemplates {
				b.WriteString("  <templates>\n")
				hasTemplates = true
			}
			fmt.Fprintf(&b, "    <template type=%q><![CDATA[\n%s\n    ]]></template>\n",
				t.Name, strings.TrimSpace(t.Template))
		}
	}
	if hasTemplates {
		b.WriteString("  </templates>\n")
	}

	b.WriteString("</context>")
	return b.String()
}

// Extract runs the CLI extract command. Options control scope selection,
// prompt input, and output destination. Empty option fields fall through
// to interactive selection.
func Extract(ws *WorkspaceSession, issueKey string, opts ExtractOptions) error {
	items, err := ws.Provider.Search(context.TODO(), "active", false)
	if err != nil {
		return fmt.Errorf("fetching workspace data: %w", err)
	}

	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)

	// Determine scope.
	var scopeName string
	switch {
	case opts.Scope != "":
		s, err := ResolveScopeName(opts.Scope)
		if err != nil {
			return err
		}
		scopeName = s
	case issueKey == "":
		scopeName = ScopeEntireWorkspace
	default:
		target := registry[issueKey]
		if target == nil {
			return fmt.Errorf("issue %s not found", issueKey)
		}
		options := ScopeOptions(target.ParentID != "")
		choice, err := ws.Runtime.UI.Select(fmt.Sprintf("LLM Extract: %s", issueKey), options)
		if err != nil {
			return err
		}
		if choice < 0 {
			return &CancelledError{Operation: "extract"}
		}
		scopeName = options[choice]
	}

	// Validate issueKey exists when a non-workspace scope requires it.
	if issueKey != "" {
		if registry[issueKey] == nil {
			return fmt.Errorf("issue %s not found", issueKey)
		}
	}

	collected := CollectExtractKeys(issueKey, scopeName, registry)

	// Determine prompt.
	prompt := opts.Prompt
	if prompt == "" {
		delimiter := "_END_OF_PROMPT_"
		boilerplate := fmt.Sprintf("\n\n%s\nType your LLM prompt above. XML context will append automatically.\n", delimiter)
		raw, err := ws.Runtime.UI.EditText(boilerplate, "llm_prompt_", 1, "")
		if err != nil {
			return fmt.Errorf("opening editor: %w", err)
		}
		prompt = strings.TrimSpace(strings.SplitN(raw, delimiter, 2)[0])
		if prompt == "" {
			return &CancelledError{Operation: "extract"}
		}
	}

	output := BuildExtractXML(prompt, collected, registry, ws.Workspace, ws.Provider.FieldDefinitions())

	if opts.Copy {
		if err := ws.Runtime.UI.CopyToClipboard(output); err != nil {
			return fmt.Errorf("copying to clipboard: %w", err)
		}
		ws.Runtime.UI.Notify("LLM Ready", fmt.Sprintf("Copied XML context (%d issues) to clipboard!", len(collected)))
	} else {
		_, _ = fmt.Fprint(ws.Runtime.Out, output)
	}

	return nil
}
