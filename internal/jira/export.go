package jira

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/client"
	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/document"
)

type ExportIssue struct {
	Key         string         `json:"key"`
	Type        string         `json:"type"`
	Summary     string         `json:"summary"`
	Status      string         `json:"status"`
	Description string         `json:"description"`
	Children    []*ExportIssue `json:"children,omitempty"`
}

type ExportMetadata struct {
	BoardSlug  string `json:"board_slug"`
	ProjectKey string `json:"project_key"`
	ExportedAt string `json:"exported_at"`
}

// BuildExportHierarchy creates a nested tree from typed issues, with per-issue hashes.
func BuildExportHierarchy(issues []client.Issue) ([]*ExportIssue, map[string]string) {
	type entry struct {
		data   *ExportIssue
		parent string
	}

	reg := make(map[string]*entry, len(issues))
	hashes := make(map[string]string, len(issues))

	for _, iss := range issues {
		f := &iss.Fields

		descMD := ""
		if len(f.Description) > 0 && string(f.Description) != "null" {
			if ast, err := document.ParseADF(f.Description); err == nil {
				descMD = strings.TrimSpace(document.RenderMarkdown(ast))
			}
		}

		parentKey := ""
		if f.Parent != nil {
			parentKey = f.Parent.Key
		}

		ei := &ExportIssue{
			Key:         iss.Key,
			Type:        f.IssueType.Name,
			Summary:     f.Summary,
			Status:      f.Status.Name,
			Description: descMD,
		}

		hashes[iss.Key] = hashExportIssue(ei)
		reg[iss.Key] = &entry{data: ei, parent: parentKey}
	}

	var roots []*ExportIssue
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

func BuildExportMetadata(slug string, board *config.BoardConfig) ExportMetadata {
	return ExportMetadata{
		BoardSlug:  slug,
		ProjectKey: board.ProjectKey,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func hashExportIssue(ei *ExportIssue) string {
	payload := map[string]string{
		"key": ei.Key, "type": ei.Type,
		"summary": ei.Summary, "status": ei.Status,
		"description": ei.Description,
	}
	data, _ := json.Marshal(payload)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
