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
	"github.com/mikecsmith/ihj/internal/encoding"
)

// ── Options ─────────────────────────────────────────────────────

// ExtractOptions controls the extract command behaviour. Empty fields
// fall through to interactive selection.
type ExtractOptions struct {
	Scope  string // Short scope name: "selected", "children", "parent", "family", "workspace". Empty = interactive.
	Preset string // Preset name: "refine", "triage", "bare". Empty = interactive.
	Prompt string // Inline prompt text. Empty = open editor.
	Copy   bool   // If true, copy to clipboard instead of writing to stdout.
	Filter string // Search filter to use. Empty defaults to "active".
}

// ── Extract presets ─────────────────────────────────────────────

// ExtractPreset controls which XML sections are included in the extract
// output and what guidance is provided to the LLM.
type ExtractPreset struct {
	Name             string // CLI flag value: "refine", "triage", "bare".
	Label            string // User-facing label for interactive selection.
	Guidance         string // LLM guidance text. Empty = no guidance section.
	IncludeFormat    bool   // Include the output format schema section.
	IncludeTemplates bool   // Include type templates section.
}

// DefaultRefineGuidance is the built-in guidance for the refine preset.
const DefaultRefineGuidance = `- This is an interactive conversation. Ask clarifying questions before producing output.
- Ask the user if they have supporting materials to share — meeting transcripts, discovery documents, proposals, specs, or design docs can dramatically improve output quality.
- Once you understand the scope, produce a brief plan and wait for confirmation before generating the structured YAML output.
- Preserve all existing issue keys exactly as provided.
- Do not invent new issue keys — if new issues are needed, omit the key field.`

// DefaultTriageGuidance is the built-in guidance for the triage preset.
const DefaultTriageGuidance = `- Review each issue and assess completeness, clarity, and priority.
- Flag issues that are too vague to estimate or implement.
- Suggest labels, priority, and grouping where missing.
- Do not create new issues or restructure the hierarchy.
- Preserve all existing issue keys exactly as provided.`

// BuiltInPresets defines the presets available out of the box.
// Workspace config can override the guidance on any built-in preset.
var BuiltInPresets = []ExtractPreset{
	{
		Name:             "refine",
		Label:            "Refine — restructure and break down",
		Guidance:         DefaultRefineGuidance,
		IncludeFormat:    true,
		IncludeTemplates: true,
	},
	{
		Name:             "triage",
		Label:            "Triage — assess and categorise",
		Guidance:         DefaultTriageGuidance,
		IncludeFormat:    true,
		IncludeTemplates: true,
	},
	{
		Name:  "bare",
		Label: "Bare context — just the issues",
	},
}

// ResolvePreset returns the preset matching the given name, applying any
// workspace-level guidance override. Returns an error for unknown names.
func ResolvePreset(name string, workspace *core.Workspace) (ExtractPreset, error) {
	for _, preset := range BuiltInPresets {
		if preset.Name == name {
			return applyGuidanceOverride(preset, workspace), nil
		}
	}
	validNames := make([]string, 0, len(BuiltInPresets))
	for _, preset := range BuiltInPresets {
		validNames = append(validNames, preset.Name)
	}
	return ExtractPreset{}, fmt.Errorf("invalid preset %q, valid values: %s", name, strings.Join(validNames, ", "))
}

// applyGuidanceOverride replaces the preset's default guidance with the
// workspace-level per-preset override when configured. Only presets with
// built-in guidance (refine, triage) can be overridden.
func applyGuidanceOverride(preset ExtractPreset, workspace *core.Workspace) ExtractPreset {
	if override, ok := workspace.ExtractGuidance[preset.Name]; ok && preset.Guidance != "" {
		preset.Guidance = override
	}
	return preset
}

// presetLabels returns the display labels for all built-in presets.
func presetLabels() []string {
	labels := make([]string, len(BuiltInPresets))
	for idx, preset := range BuiltInPresets {
		labels[idx] = preset.Label
	}
	return labels
}

// ── Scope constants and resolution ──────────────────────────────

const (
	ScopeSelectedOnly    = "Just this issue"
	ScopeWithChildren    = "This issue and its sub-issues"
	ScopeWithParent      = "This issue and its parent"
	ScopeFullFamily      = "Full hierarchy — parent, siblings, and all sub-issues"
	ScopeEntireWorkspace = "Everything in the workspace"

	defaultSearchFilter = "active"
)

// scopeShortNames maps CLI-friendly short names to the internal scope constants.
var scopeShortNames = map[string]string{
	"selected":  ScopeSelectedOnly,
	"children":  ScopeWithChildren,
	"parent":    ScopeWithParent,
	"family":    ScopeFullFamily,
	"workspace": ScopeEntireWorkspace,
}

// ResolveScopeName maps a short CLI scope name to the internal scope constant.
func ResolveScopeName(shortName string) (string, error) {
	if resolved, ok := scopeShortNames[shortName]; ok {
		return resolved, nil
	}
	validNames := make([]string, 0, len(scopeShortNames))
	for name := range scopeShortNames {
		validNames = append(validNames, name)
	}
	sort.Strings(validNames)
	return "", fmt.Errorf("invalid scope %q, valid values: %s", shortName, strings.Join(validNames, ", "))
}

// ScopeOptions returns the available scope options for the given issue.
func ScopeOptions(hasParent bool) []string {
	options := []string{ScopeSelectedOnly, ScopeWithChildren}
	if hasParent {
		options = append(options, ScopeWithParent, ScopeFullFamily)
	}
	options = append(options, ScopeEntireWorkspace)
	return options
}

// ── Extract command ─────────────────────────────────────────────

// Extract runs the CLI extract command. Options control scope selection,
// preset, prompt input, and output destination. Empty option fields fall
// through to interactive selection.
func Extract(ctx context.Context, session *WorkspaceSession, issueKey string, opts ExtractOptions) error {
	provider := session.Provider
	ui := session.Runtime.UI
	workspace := session.Workspace

	preset, err := resolvePresetSelection(ui, opts.Preset, workspace)
	if err != nil {
		return err
	}

	registry, err := fetchRegistry(ctx, provider, opts.Filter)
	if err != nil {
		return err
	}

	scopeName, err := resolveScope(ui, issueKey, opts.Scope, registry)
	if err != nil {
		return err
	}

	if issueKey != "" && registry[issueKey] == nil {
		return fmt.Errorf("issue %s not found", issueKey)
	}

	collectedKeys := CollectExtractKeys(issueKey, scopeName, registry)

	prompt, err := resolvePrompt(ui, opts.Prompt)
	if err != nil {
		return err
	}

	output := BuildExtractXML(prompt, preset, collectedKeys, registry, workspace, workspace.AllFieldDefs())

	return deliverOutput(session, output, len(collectedKeys), opts.Copy)
}

// resolvePresetSelection determines the extract preset from CLI options or
// interactive selection.
func resolvePresetSelection(ui UI, optPreset string, workspace *core.Workspace) (ExtractPreset, error) {
	if optPreset != "" {
		return ResolvePreset(optPreset, workspace)
	}

	labels := presetLabels()
	choice, err := ui.Select("Extract mode", labels)
	if err != nil {
		return ExtractPreset{}, err
	}
	if choice < 0 {
		return ExtractPreset{}, &CancelledError{Operation: "extract"}
	}

	return applyGuidanceOverride(BuiltInPresets[choice], workspace), nil
}

// fetchRegistry loads workspace issues and builds a linked registry.
func fetchRegistry(ctx context.Context, provider core.Provider, filter string) (map[string]*core.WorkItem, error) {
	if filter == "" {
		filter = defaultSearchFilter
	}
	items, err := provider.Search(ctx, filter, false)
	if err != nil {
		return nil, fmt.Errorf("fetching workspace data: %w", err)
	}
	registry := core.BuildRegistry(items)
	core.LinkChildren(registry)
	return registry, nil
}

// resolveScope determines the extraction scope from CLI options or interactive selection.
func resolveScope(ui UI, issueKey, optScope string, registry map[string]*core.WorkItem) (string, error) {
	switch {
	case optScope != "":
		return ResolveScopeName(optScope)

	case issueKey == "":
		return ScopeEntireWorkspace, nil

	default:
		target := registry[issueKey]
		if target == nil {
			return "", fmt.Errorf("issue %s not found", issueKey)
		}
		scopeOptions := ScopeOptions(target.ParentID != "")
		choice, err := ui.Select(fmt.Sprintf("LLM Extract: %s", issueKey), scopeOptions)
		if err != nil {
			return "", err
		}
		if choice < 0 {
			return "", &CancelledError{Operation: "extract"}
		}
		return scopeOptions[choice], nil
	}
}

// resolvePrompt determines the LLM prompt from CLI options or interactive input.
func resolvePrompt(ui UI, optPrompt string) (string, error) {
	if optPrompt != "" {
		return optPrompt, nil
	}
	raw, err := ui.InputText("LLM Prompt (XML context appended automatically)", "")
	if err != nil {
		return "", err
	}
	prompt := strings.TrimSpace(raw)
	if prompt == "" {
		return "", &CancelledError{Operation: "extract"}
	}
	return prompt, nil
}

// deliverOutput writes the extract output to clipboard or stdout.
func deliverOutput(session *WorkspaceSession, output string, issueCount int, copyToClipboard bool) error {
	ui := session.Runtime.UI
	if copyToClipboard {
		if err := ui.CopyToClipboard(output); err != nil {
			return fmt.Errorf("copying to clipboard: %w", err)
		}
		ui.Notify("LLM Ready", fmt.Sprintf("Copied XML context (%d issues) to clipboard!", issueCount))
		return nil
	}
	_, _ = fmt.Fprint(session.Runtime.Out, output)
	return nil
}

// ── Key collection ──────────────────────────────────────────────

// CollectExtractKeys determines which issue keys to include based on scope,
// working from the WorkItem registry. Used by both CLI and TUI.
func CollectExtractKeys(issueKey, scope string, registry map[string]*core.WorkItem) map[string]bool {
	// Entire workspace doesn't need a target issue.
	if scope == ScopeEntireWorkspace {
		collected := make(map[string]bool, len(registry))
		for key := range registry {
			collected[key] = true
		}
		return collected
	}

	collected := map[string]bool{issueKey: true}
	target := registry[issueKey]
	if target == nil {
		return collected
	}

	switch scope {
	case ScopeSelectedOnly:
		// Just the target.

	case ScopeWithChildren:
		addChildren(collected, target)

	case ScopeWithParent:
		if target.ParentID != "" {
			collected[target.ParentID] = true
		}

	case ScopeFullFamily:
		addChildren(collected, target)
		addFamilyTree(collected, target, registry)
	}

	return collected
}

// addChildren adds all direct children of the target to the collected set.
func addChildren(collected map[string]bool, target *core.WorkItem) {
	for _, child := range target.Children {
		collected[child.ID] = true
	}
}

// addFamilyTree adds the parent, siblings, and their children (nieces/nephews)
// to the collected set.
func addFamilyTree(collected map[string]bool, target *core.WorkItem, registry map[string]*core.WorkItem) {
	if target.ParentID == "" {
		return
	}
	collected[target.ParentID] = true

	parent, ok := registry[target.ParentID]
	if !ok {
		return
	}
	for _, sibling := range parent.Children {
		collected[sibling.ID] = true
		if siblingItem, ok := registry[sibling.ID]; ok {
			for _, nephew := range siblingItem.Children {
				collected[nephew.ID] = true
			}
		}
	}
}

// ── XML generation ──────────────────────────────────────────────

// BuildExtractXML produces the XML context for an LLM prompt from WorkItem data.
// The preset controls which sections (guidance, output format, templates) are
// included. Used by both CLI and TUI extract flows.
func BuildExtractXML(prompt string, preset ExtractPreset, issueKeys map[string]bool, registry map[string]*core.WorkItem, workspace *core.Workspace, fieldDefs []core.FieldDef) string {
	var buf strings.Builder

	writeInstruction(&buf, prompt)
	writeGuidance(&buf, preset.Guidance)
	if preset.IncludeFormat {
		writeOutputFormat(&buf, workspace, fieldDefs)
	}
	typesUsed := writeIssues(&buf, issueKeys, registry, fieldDefs)
	if preset.IncludeTemplates {
		writeTemplates(&buf, workspace, typesUsed)
	}

	buf.WriteString("</context>")
	return buf.String()
}

func writeInstruction(buf *strings.Builder, prompt string) {
	buf.WriteString("<context>\n  <instruction>\n    ")
	buf.WriteString(xmlEscape(prompt))
	buf.WriteString("\n  </instruction>\n")
}

func writeGuidance(buf *strings.Builder, guidance string) {
	if guidance == "" {
		return
	}
	buf.WriteString("  <guidance>\n    ")
	buf.WriteString(strings.ReplaceAll(guidance, "\n", "\n    "))
	buf.WriteString("\n  </guidance>\n")
}

func writeOutputFormat(buf *strings.Builder, workspace *core.Workspace, fieldDefs []core.FieldDef) {
	schema := encoding.ManifestSchema(workspace, fieldDefs)
	schemaJSON, _ := json.MarshalIndent(schema, "    ", "  ")
	buf.WriteString("  <output_format>\n")
	buf.WriteString("    Output as structured YAML validating against this schema:\n")
	buf.WriteString("    <json_schema>\n    ")
	buf.Write(schemaJSON)
	buf.WriteString("\n    </json_schema>\n  </output_format>\n")
}

func writeIssues(buf *strings.Builder, issueKeys map[string]bool, registry map[string]*core.WorkItem, fieldDefs []core.FieldDef) map[string]bool {
	buf.WriteString("  <issues>\n")
	typesUsed := make(map[string]bool)

	for _, issueKey := range sortedKeys(issueKeys) {
		issue, ok := registry[issueKey]
		if !ok {
			continue
		}
		typesUsed[issue.Type] = true
		writeIssueElement(buf, issueKey, issue, fieldDefs)
	}

	buf.WriteString("  </issues>\n")
	return typesUsed
}

func writeIssueElement(buf *strings.Builder, issueKey string, issue *core.WorkItem, fieldDefs []core.FieldDef) {
	fmt.Fprintf(buf, "    <issue key=%q type=%q status=%q", issueKey, issue.Type, issue.Status)
	if issue.ParentID != "" {
		fmt.Fprintf(buf, " parent=%q", issue.ParentID)
	}
	buf.WriteString(">\n")
	fmt.Fprintf(buf, "      <summary>%s</summary>\n", xmlEscape(issue.Summary))

	writeFieldElements(buf, issue, fieldDefs)

	if issue.Description != nil {
		descriptionMarkdown := strings.TrimSpace(document.RenderMarkdown(issue.Description))
		if descriptionMarkdown != "" {
			fmt.Fprintf(buf, "      <description><![CDATA[\n%s\n      ]]></description>\n", descriptionMarkdown)
		}
	}
	buf.WriteString("    </issue>\n")
}

func writeFieldElements(buf *strings.Builder, issue *core.WorkItem, fieldDefs []core.FieldDef) {
	for _, def := range fieldDefs {
		if !def.Diffable() {
			continue
		}
		fieldValue, ok := issue.Fields[def.Key]
		if !ok || core.IsZeroFieldValue(fieldValue) {
			continue
		}
		switch typedValue := fieldValue.(type) {
		case []string:
			fmt.Fprintf(buf, "      <%s>%s</%s>\n", def.Key, xmlEscape(strings.Join(typedValue, ", ")), def.Key)
		default:
			fmt.Fprintf(buf, "      <%s>%s</%s>\n", def.Key, xmlEscape(fmt.Sprintf("%v", typedValue)), def.Key)
		}
	}
}

func writeTemplates(buf *strings.Builder, workspace *core.Workspace, typesUsed map[string]bool) {
	hasTemplates := false
	for _, typeDef := range workspace.Types {
		if typesUsed[typeDef.Name] && typeDef.Template != "" {
			if !hasTemplates {
				buf.WriteString("  <templates>\n")
				hasTemplates = true
			}
			fmt.Fprintf(buf, "    <template type=%q><![CDATA[\n%s\n    ]]></template>\n",
				typeDef.Name, strings.TrimSpace(typeDef.Template))
		}
	}
	if hasTemplates {
		buf.WriteString("  </templates>\n")
	}
}

// ── Helpers ─────────────────────────────────────────────────────

// xmlEscape returns the input with XML special characters escaped.
func xmlEscape(input string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(input))
	return buf.String()
}

// sortedKeys returns the keys of a map[string]bool in sorted order.
func sortedKeys(keySet map[string]bool) []string {
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
