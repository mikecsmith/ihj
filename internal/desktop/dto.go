package desktop

import (
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// viewItem wraps core.WorkItem for JSON serialization to the frontend.
// Core types carry json tags for scalar fields. This wrapper only exists
// because Description and Comments are AST nodes that need rendering to
// markdown strings, and Children need recursive wrapping.
type viewItem struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Summary       string         `json:"summary"`
	Status        string         `json:"status"`
	ParentID      string         `json:"parentId"`
	Description   string         `json:"description"`
	Fields        map[string]any `json:"fields"`
	DisplayFields map[string]any `json:"displayFields"`
	Children      []viewItem     `json:"children"`
	Comments      []viewComment  `json:"comments"`
}

type viewComment struct {
	Author  string `json:"author"`
	Created string `json:"created"`
	Body    string `json:"body"`
}

// toView converts a core.WorkItem tree into viewItems for the frontend.
func toView(items []*core.WorkItem) []viewItem {
	out := make([]viewItem, 0, len(items))
	for _, item := range items {
		v := viewItem{
			ID:            item.ID,
			Type:          item.Type,
			Summary:       item.Summary,
			Status:        item.Status,
			ParentID:      item.ParentID,
			Description:   item.DescriptionMarkdown(),
			Fields:        item.Fields,
			DisplayFields: item.DisplayFields,
			Children:      toView(item.Children),
		}
		if v.Fields == nil {
			v.Fields = make(map[string]any)
		}
		if v.DisplayFields == nil {
			v.DisplayFields = make(map[string]any)
		}

		v.Comments = make([]viewComment, 0, len(item.Comments))
		for _, c := range item.Comments {
			body := ""
			if c.Body != nil {
				body = document.RenderMarkdown(c.Body)
			}
			v.Comments = append(v.Comments, viewComment{
				Author:  c.Author,
				Created: c.Created,
				Body:    body,
			})
		}

		out = append(out, v)
	}
	return out
}
