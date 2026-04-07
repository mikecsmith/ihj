// field_meta.go — Field metadata discovery, createmeta loading, and
// per-type FieldDef assembly.

package jira

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
)

// MetaCacheRefreshThreshold is the fraction of DefaultMetaCacheTTL after
// which a background refresh is triggered. At 0.8, a 24h TTL triggers
// refresh after ~19h so the cache is warm before it expires.
const MetaCacheRefreshThreshold = 0.8

// loadFieldMeta fetches createmeta data (disk cache → API), merges it with
// the global hardcoded fields, and populates per-type FieldDefs on TypeConfig.
// Also builds the nameToID lookup table. The workspace-wide union is derived
// on demand via ws.AllFieldDefs().
func (p *Provider) loadFieldMeta() error {
	if p.cacheDir == "" || p.cfg == nil {
		return fmt.Errorf("no cache dir or config")
	}

	meta, err := p.resolveCreateMeta()
	if err != nil {
		return err
	}

	globals := p.wellKnown.ToFieldDefs()
	p.nameToID = make(map[string]string)

	for i := range p.ws.Types {
		tc := &p.ws.Types[i]
		typeID := fmt.Sprintf("%d", tc.ID)

		metaFields, ok := meta.Types[typeID]
		if !ok {
			tc.Fields = globals
			continue
		}

		// Build a lookup of createmeta fields by fieldId.
		metaByID := make(map[string]createMetaField, len(metaFields))
		for _, mf := range metaFields {
			metaByID[mf.FieldID] = mf
		}

		// Start with globals, patching enums from createmeta.
		typeDefs := make(core.FieldDefs, len(globals))
		copy(typeDefs, globals)
		p.linkGlobalsToMeta(typeDefs, metaByID)

		// Build set of field IDs that have a workspace alias or per-type
		// override so the general sweep doesn't add a duplicate raw key.
		aliasedIDs := make(map[string]bool)
		for _, cfID := range p.ws.FieldAliases {
			aliasedIDs[fmt.Sprintf("customfield_%d", cfID)] = true
		}
		for _, cfID := range tc.ExtraFields {
			aliasedIDs[fmt.Sprintf("customfield_%d", cfID)] = true
		}

		// Add workspace-wide field alias entries first (Pinned=true, friendly key).
		for alias, cfID := range p.ws.FieldAliases {
			fieldID := fmt.Sprintf("customfield_%d", cfID)
			if mf, ok := metaByID[fieldID]; ok {
				if typeDefs.WithKey(alias) == nil {
					def := metaFieldToDef(mf, true)
					def.Key = alias // use the config alias as key
					p.wellKnown.ApplyOverrides(&def)
					typeDefs = append(typeDefs, def)
				}
			}
		}

		// Add per-type ExtraFields entries (Pinned=true).
		for alias, cfID := range tc.ExtraFields {
			fieldID := fmt.Sprintf("customfield_%d", cfID)
			if mf, ok := metaByID[fieldID]; ok {
				if typeDefs.WithKey(alias) == nil {
					def := metaFieldToDef(mf, true)
					def.Key = alias
					p.wellKnown.ApplyOverrides(&def)
					typeDefs = append(typeDefs, def)
				}
			}
		}

		// Add remaining non-global createmeta fields. Key is derived from
		// the Jira field name (e.g. "Epic Link" → "epic_link"). On collision
		// the numeric custom field ID is appended (e.g. "team_20001").
		// Skip aliased fields to prevent duplicate entries.
		for _, mf := range metaFields {
			if p.isExcludedField(mf.FieldID) || aliasedIDs[mf.FieldID] {
				continue
			}
			if !isKnownCustomType(mf.Schema.Custom) {
				continue
			}
			def := metaFieldToDef(mf, false)
			if key := nameToKey(mf.Name); key != "" {
				def.Key = key
			}
			p.wellKnown.ApplyOverrides(&def)
			if typeDefs.WithKey(def.Key) != nil {
				def.Key = def.Key + "_" + strings.TrimPrefix(mf.FieldID, "customfield_")
			}
			if typeDefs.WithKey(def.Key) != nil {
				continue // still collides — skip
			}
			typeDefs = append(typeDefs, def)
		}

		tc.Fields = typeDefs
	}

	return nil
}

// backgroundRefreshIfNeeded checks the disk cache age and triggers a
// background API refresh if it's past MetaCacheRefreshThreshold of the TTL.
// This keeps the cache warm so the next session doesn't hit the API
// synchronously (which causes UI pop-in).
func (p *Provider) backgroundRefreshIfNeeded() {
	if p.cacheDir == "" || p.cfg == nil {
		return
	}
	path := createMetaCachePath(p.cacheDir, p.ws.Slug)
	info, err := os.Stat(path)
	if err != nil {
		return // no cache file — was just fetched fresh, nothing to refresh
	}
	threshold := time.Duration(float64(DefaultMetaCacheTTL) * MetaCacheRefreshThreshold)
	if time.Since(info.ModTime()) < threshold {
		return // cache is fresh enough
	}

	go func() {
		ctx := context.Background()
		meta := &cachedCreateMeta{
			Types: make(map[string][]createMetaField),
		}
		for _, tc := range p.ws.Types {
			typeID := fmt.Sprintf("%d", tc.ID)
			fields, err := p.client.FetchCreateMetaFields(ctx, p.cfg.ProjectKey, typeID)
			if err != nil {
				return // silently abandon — current cache is still valid
			}
			meta.Types[typeID] = fields
		}
		_ = saveCreateMetaCache(p.cacheDir, p.ws.Slug, meta)
	}()
}

// resolveCreateMeta loads createmeta from disk cache or fetches from the API.
func (p *Provider) resolveCreateMeta() (*cachedCreateMeta, error) {
	slug := p.ws.Slug
	project := p.cfg.ProjectKey

	// Try disk cache first.
	if cached, err := loadCreateMetaCache(p.cacheDir, slug, DefaultMetaCacheTTL); err == nil {
		return cached, nil
	}

	// No cache — fetch from API. Print a status line so the user knows
	// why there's a brief pause on first run.
	fmt.Fprintf(os.Stderr, "Loading field metadata for %s…\n", project)

	ctx := context.Background()
	meta := &cachedCreateMeta{
		Types: make(map[string][]createMetaField),
	}

	for _, tc := range p.ws.Types {
		typeID := fmt.Sprintf("%d", tc.ID)
		fields, err := p.client.FetchCreateMetaFields(ctx, project, typeID)
		if err != nil {
			// Graceful fallback: if createmeta is unavailable for any type, abort.
			return nil, fmt.Errorf("fetching createmeta for type %s (%s): %w", typeID, tc.Name, err)
		}
		meta.Types[typeID] = fields
	}

	// Persist to disk.
	_ = saveCreateMetaCache(p.cacheDir, slug, meta)
	return meta, nil
}

// linkGlobalsToMeta populates well-known global FieldDefs with runtime data
// from createmeta: priority enum values + nameToID lookup, sprint FieldID,
// team FieldID. Called per-type to build type-specific copies.
func (p *Provider) linkGlobalsToMeta(defs core.FieldDefs, metaByID map[string]createMetaField) {
	for i := range defs {
		switch defs[i].Key {
		case "priority":
			if mf, ok := metaByID["priority"]; ok && len(mf.AllowedValues) > 0 {
				names, ids := extractAllowedValues(mf.AllowedValues)
				if len(names) > 0 {
					defs[i].Enum = names
					for j, name := range names {
						p.nameToID["priority:"+name] = ids[j]
					}
				}
			}
		case "sprint":
			// Link sprint to its Jira custom field ID so the search API
			// requests it and the registry can extract the active sprint name.
			for _, mf := range metaByID {
				if mf.Schema.Custom == "com.pyxis.greenhopper.jira:gh-sprint" {
					defs[i].FieldID = mf.FieldID
					defs[i].Icon = core.IconSprint
					break
				}
			}
		case "team":
			// Link team to its Jira custom field ID via workspace FieldAliases.
			if cfID, ok := p.ws.FieldAliases["team"]; ok {
				fieldID := fmt.Sprintf("customfield_%d", cfID)
				if _, ok := metaByID[fieldID]; ok {
					defs[i].FieldID = fieldID
					defs[i].Pinned = true
				}
			}
		}
	}
}

// isExcludedField returns true if the field ID should never be captured
// as a FieldDef. Delegates to the well-known field registry.
func (p *Provider) isExcludedField(fieldID string) bool {
	wk, ok := p.wellKnown[fieldID]
	return ok && wk.Excluded
}

// customFieldIDs returns the Jira field IDs (e.g. "customfield_10016") for
// all dynamic fields discovered via createmeta. Collects from per-type
// FieldDefs (not the union) so that different types mapping different field
// IDs to the same alias all get requested.
func (p *Provider) customFieldIDs() []string {
	seen := make(map[string]bool)
	var ids []string
	for _, tc := range p.ws.Types {
		for _, d := range tc.Fields {
			if d.FieldID != "" && !p.isExcludedField(d.FieldID) && !seen[d.FieldID] {
				seen[d.FieldID] = true
				ids = append(ids, d.FieldID)
			}
		}
	}
	return ids
}

// customFieldMap returns a mapping of Jira field ID → binding for all
// dynamic fields across all types. Extraction is intentionally broad —
// issues may carry field values that createmeta doesn't list for their
// type. Display-time filtering (via TypeConfig.Fields) controls visibility.
func (p *Provider) customFieldMap() map[string]customFieldBinding {
	m := make(map[string]customFieldBinding)
	for _, tc := range p.ws.Types {
		for _, d := range tc.Fields {
			if d.FieldID != "" && !p.isExcludedField(d.FieldID) {
				m[d.FieldID] = customFieldBinding{Alias: d.Key, Type: d.Type}
			}
		}
	}
	return m
}

// customFieldBinding pairs a Jira field ID with its alias key and the
// core field type, so the registry can decide how to decode the value
// (plain string vs ADF rich text).
type customFieldBinding struct {
	Alias string
	Type  core.FieldType
}
