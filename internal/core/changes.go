package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/mikecsmith/ihj/internal/document"
)

// FieldPresence records which keys were explicitly present in a decoded
// payload. Consumers use it to distinguish intent for each field:
//
//   - key in FieldPresence, zero value     → Clear (user set to empty)
//   - key in FieldPresence, non-zero value → Set (user provided value)
//   - key not in FieldPresence             → Omit (user did not touch)
//
// The presence signal cannot be recovered from the WorkItem alone once
// zero values are indistinguishable from omitted fields — decoders must
// build FieldPresence alongside the WorkItem they produce.
type FieldPresence map[string]bool

// Has reports whether key was present in the source payload.
func (s FieldPresence) Has(key string) bool { return s != nil && s[key] }

// ComputeChanges produces the delta between orig (the current state) and
// edited (the user's new state), using set as the presence oracle. Fields
// not marked in set are treated as omit — no change is emitted. Empty
// values marked in set become clear intents.
//
// Identity-required core keys (summary, type) cannot be cleared; an attempt
// to do so returns an error. Status clears are deferred to provider
// validation — ComputeChanges emits whatever the user set.
func ComputeChanges(orig, edited *WorkItem, set FieldPresence, defs FieldDefs) (*Changes, error) {
	if set.Has(KeySummary) && edited.Summary == "" {
		return nil, fmt.Errorf("summary cannot be cleared — it is required")
	}
	if set.Has(KeyType) && edited.Type == "" {
		return nil, fmt.Errorf("type cannot be cleared — it is required")
	}

	ch := &Changes{}
	hasChange := false

	if set.Has(KeySummary) && edited.Summary != orig.Summary {
		v := edited.Summary
		ch.Summary = &v
		hasChange = true
	}
	if set.Has(KeyType) && !strings.EqualFold(edited.Type, orig.Type) {
		v := edited.Type
		ch.Type = &v
		hasChange = true
	}
	if set.Has(KeyStatus) && !strings.EqualFold(edited.Status, orig.Status) {
		v := edited.Status
		ch.Status = &v
		hasChange = true
	}
	if set.Has(KeyParent) && edited.ParentID != orig.ParentID {
		v := edited.ParentID
		ch.ParentID = &v
		hasChange = true
	}
	if set.Has(KeyDescription) {
		newMD := RenderRichText(edited.Description)
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
		if fieldValuesEqual(editedV, origV, def.Type) {
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

// fieldValuesEqual compares two field values by FieldType, applying
// type-aware equality: RichText compares via rendered markdown (AST equality
// is unstable), StringArray is sort-independent (order is not semantic), and
// other types fall back to reflect.DeepEqual.
func fieldValuesEqual(a, b any, ft FieldType) bool {
	switch ft {
	case FieldRichText:
		return RenderRichText(a) == RenderRichText(b)
	case FieldStringArray:
		as := toStringSlice(a)
		bs := toStringSlice(b)
		sort.Strings(as)
		sort.Strings(bs)
		return slices.Equal(as, bs)
	}
	return reflect.DeepEqual(a, b)
}

// toStringSlice coerces a field value to []string, accepting []string or
// []any. Returns nil for all other types including nil.
func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		out := make([]string, len(val))
		for i, item := range val {
			out[i] = fmt.Sprintf("%v", item)
		}
		return out
	}
	return nil
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
		KeyParent:      parentID,
		KeyType:        item.Type,
		KeySummary:     item.Summary,
		KeyDescription: item.DescriptionMarkdown(),
		KeyFields:      filtered,
	}
	data, _ := json.Marshal(payload)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
