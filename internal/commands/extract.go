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
)

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

	issueMap := loadCachedIssueMap(app.CacheDir, boardSlug)
	target, ok := issueMap[issueKey]
	if !ok {
		return fmt.Errorf("issue %s not found in local cache", issueKey)
	}

	parentKey := ""
	if target.Fields.Parent != nil {
		parentKey = target.Fields.Parent.Key
	}

	options := []string{"Target Issue Only", "Target + Children"}
	if parentKey != "" {
		options = append(options, "Target + Parent", "Full Family (Parent + Siblings + Children)")
	}

	choice, err := app.UI.Select(fmt.Sprintf("LLM Extract: %s", issueKey), options)
	if err != nil {
		return err
	}
	if choice < 0 {
		return &CancelledError{Operation: "extract"}
	}

	collected := collectIssueKeys(issueKey, parentKey, options[choice], issueMap)

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

	xml := buildXMLContext(prompt, collected, issueMap, boardCfg)

	if err := app.UI.CopyToClipboard(xml); err != nil {
		return fmt.Errorf("copying to clipboard: %w", err)
	}

	app.UI.Notify("LLM Ready", fmt.Sprintf("Copied XML context (%d issues) to clipboard!", len(collected)))
	return nil
}

func collectIssueKeys(issueKey, parentKey, mode string, issues map[string]client.Issue) map[string]bool {
	keys := map[string]bool{issueKey: true}

	includeChildren := strings.Contains(mode, "Children") || strings.Contains(mode, "Family")
	includeParent := strings.Contains(mode, "Parent") || strings.Contains(mode, "Family")
	includeSiblings := strings.Contains(mode, "Family")

	if includeChildren {
		for k, iss := range issues {
			if iss.Fields.Parent != nil && iss.Fields.Parent.Key == issueKey {
				keys[k] = true
			}
		}
	}
	if includeParent && parentKey != "" {
		keys[parentKey] = true
	}
	if includeSiblings && parentKey != "" {
		for k, iss := range issues {
			if iss.Fields.Parent != nil && iss.Fields.Parent.Key == parentKey {
				keys[k] = true
			}
		}
	}
	return keys
}

func buildXMLContext(prompt string, keys map[string]bool, issues map[string]client.Issue, board *config.BoardConfig) string {
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
		iss, ok := issues[key]
		if !ok {
			continue
		}
		f := &iss.Fields
		typesUsed[f.IssueType.Name] = true

		descMD := ""
		if len(f.Description) > 0 && string(f.Description) != "null" {
			if ast, err := document.ParseADF(f.Description); err == nil {
				descMD = strings.TrimSpace(document.RenderMarkdown(ast))
			}
		}

		fmt.Fprintf(&b, "    <issue key=%q type=%q status=%q>\n", key, f.IssueType.Name, f.Status.Name)
		fmt.Fprintf(&b, "      <summary>%s</summary>\n", f.Summary)
		if descMD != "" {
			fmt.Fprintf(&b, "      <description>\n%s\n      </description>\n", descMD)
		}
		b.WriteString("    </issue>\n")
	}
	b.WriteString("  </issues>\n")

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
