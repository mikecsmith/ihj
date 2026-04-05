package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/mikecsmith/ihj/internal/document"
)

// SetKeys records which keys were explicitly present in a decoded payload.
// Consumers use it to distinguish intent for each field:
//
//   - key in SetKeys, zero value     → Clear (user set to empty)
//   - key in SetKeys, non-zero value → Set (user provided value)
//   - key not in SetKeys             → Omit (user did not touch)
//
// The presence signal cannot be recovered from the WorkItem alone once
// zero values are indistinguishable from omitted fields — decoders must
// build SetKeys alongside the WorkItem they produce.
type SetKeys map[string]bool

// Has reports whether key was present in the source payload.
func (s SetKeys) Has(key string) bool { return s != nil && s[key] }

// ComputeChanges produces the delta between orig (the current state) and
// edited (the user's new state), using set as the presence oracle. Fields
// not marked in set are treated as omit — no change is emitted. Empty
// values marked in set become clear intents.
//
// Identity-required core keys (summary, type) cannot be cleared; an attempt
// to do so returns an error. Status clears are deferred to provider
// validation — ComputeChanges emits whatever the user set.
func ComputeChanges(orig, edited *WorkItem, set SetKeys, defs FieldDefs) (*Changes, error) {
	if set.Has("summary") && edited.Summary == "" {
		return nil, fmt.Errorf("summary cannot be cleared — it is required")
	}
	if set.Has("type") && edited.Type == "" {
		return nil, fmt.Errorf("type cannot be cleared — it is required")
	}

	ch := &Changes{}
	hasChange := false

	if set.Has("summary") && edited.Summary != orig.Summary {
		v := edited.Summary
		ch.Summary = &v
		hasChange = true
	}
	if set.Has("type") && !strings.EqualFold(edited.Type, orig.Type) {
		v := edited.Type
		ch.Type = &v
		hasChange = true
	}
	if set.Has("status") && !strings.EqualFold(edited.Status, orig.Status) {
		v := edited.Status
		ch.Status = &v
		hasChange = true
	}
	if set.Has("parent") && edited.ParentID != orig.ParentID {
		v := edited.ParentID
		ch.ParentID = &v
		hasChange = true
	}
	if set.Has("description") {
		newMD := ""
		if edited.Description != nil {
			newMD = strings.TrimSpace(document.RenderMarkdown(edited.Description))
		}
		if newMD != orig.DescriptionMarkdown() {
			if edited.Description == nil {
				ch.Description = document.NewDoc()
			} else {
				ch.Description = edited.Description
			}
			hasChange = true
		}
	}

	// FieldDef-driven entries.
	fields := make(map[string]any)
	for _, def := range defs {
		if !def.Diffable() || !set.Has(def.Key) {
			continue
		}
		editedV := edited.Fields[def.Key]
		origV := orig.Fields[def.Key]

		// WriteOnly action fields emit on presence regardless of orig —
		// their write IS the intent (fire-and-forget command).
		if def.WriteOnly {
			if !IsZeroFieldValue(editedV) {
				fields[def.Key] = editedV
				hasChange = true
			}
			continue
		}

		if IsZeroFieldValue(editedV) && IsZeroFieldValue(origV) {
			continue
		}
		if reflect.DeepEqual(editedV, origV) {
			continue
		}
		fields[def.Key] = editedV
		hasChange = true
	}
	if len(fields) > 0 {
		ch.Fields = fields
	}

	if !hasChange {
		return nil, nil
	}
	return ch, nil
}

// ComputeStateHash returns a stable signature over an item's user-writable
// state for apply-retry identity. Only IncludeInSchema fields contribute —
// provider-injected Derived/Immutable fields are ignored so they cannot
// invalidate the hash between retries.
func ComputeStateHash(item *WorkItem, parentID string, defs FieldDefs) string {
	filtered := make(map[string]any)
	for _, def := range defs {
		if !def.IncludeInSchema() {
			continue
		}
		if v, ok := item.Fields[def.Key]; ok && !IsZeroFieldValue(v) {
			filtered[def.Key] = v
		}
	}
	payload := map[string]any{
		"parent":      parentID,
		"type":        item.Type,
		"summary":     item.Summary,
		"description": item.DescriptionMarkdown(),
		"fields":      filtered,
	}
	data, _ := json.Marshal(payload)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
