package jira

import (
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

// BuildExportHierarchy creates a nested tree from typed issues, with per-issue hashes.
func BuildExportHierarchy(issues []Issue) ([]*core.WorkItem, map[string]string) {
	type entry struct {
		data   *core.WorkItem
		parent string
	}

	reg := make(map[string]*entry, len(issues))
	hashes := make(map[string]string, len(issues))

	for _, iss := range issues {
		f := &iss.Fields

		parentKey := ""
		if f.Parent != nil {
			parentKey = f.Parent.Key
		}

		ewi := &core.WorkItem{
			ID:      iss.Key,
			Type:    f.IssueType.Name,
			Summary: f.Summary,
			Status:  f.Status.Name,
		}

		// Parse ADF description into AST.
		if len(f.Description) > 0 && string(f.Description) != "null" {
			ewi.Description, _ = ParseADF(f.Description)
		}

		hashes[iss.Key] = ewi.ContentHash()
		reg[iss.Key] = &entry{data: ewi, parent: parentKey}
	}

	var roots []*core.WorkItem
	for _, e := range reg {
		if e.parent != "" {
			if p, ok := reg[e.parent]; ok {
				p.data.Children = append(p.data.Children, e.data)
				continue
			}
		}
		roots = append(roots, e.data)
	}

	return roots, hashes
}

func BuildExportMetadata(slug string, cfg *Config) core.Metadata {
	return core.Metadata{
		Backend: "jira",
		Target:  slug,
		Context: map[string]any{
			"project_key": cfg.ProjectKey,
		},
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
}
