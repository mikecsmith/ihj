// Package jira implements the Atlassian Jira provider.
//
// It acts as an adapter between the Jira REST API and the universal
// domain model defined in the core package. Its primary responsibilities
// are translating Jira-specific concepts (ADF descriptions, JQL, custom
// fields, sprint management, and workflow transitions) into backend-agnostic
// core.WorkItem structures, and managing per-workspace caching.
//
// API types are derived from the Atlassian OpenAPI spec at:
//
//	https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Provider implements core.Provider for Jira backends.
// It wraps the low-level API client and translates between
// Jira-specific types and the universal core.WorkItem model.
type Provider struct {
	client   API
	ws       *core.Workspace
	cfg      *Config
	cacheDir string

	// cachedUser avoids repeated FetchMyself calls within a session.
	cachedUser *user

	// wellKnown is the single source of truth for fields the provider recognises.
	wellKnown wellKnownFields

	// metaFields is the union of all type fields, populated eagerly at construction.
	metaFields core.FieldDefs
	nameToID   map[string]string // "fieldKey:valueName" → "valueID" for payload construction
}

// Compile-time check that *Provider implements core.Provider.
var _ core.Provider = (*Provider)(nil)

// MetaCacheRefreshThreshold is the fraction of DefaultMetaCacheTTL after
// which a background refresh is triggered. At 0.8, a 24h TTL triggers
// refresh after ~19h so the cache is warm before it expires.
const MetaCacheRefreshThreshold = 0.8

// NewProvider creates a Jira provider for the given workspace.
// The workspace's ProviderConfig must already be a *jira.Config
// (hydrated by the composition root).
// Eagerly loads createmeta (from disk cache or API) so field metadata
// is available immediately. Returns an error if createmeta cannot be loaded.
// cacheDir may be empty to disable disk caching.
func NewProvider(client API, ws *core.Workspace, cacheDir string) (*Provider, error) {
	cfg, _ := ws.ProviderConfig.(*Config)
	p := &Provider{
		client:   client,
		ws:       ws,
		cfg:      cfg,
		cacheDir: cacheDir,
	}
	p.wellKnown = p.buildWellKnownFields()

	fields, err := p.loadFieldMeta()
	if err != nil {
		return nil, fmt.Errorf("loading field metadata: %w", err)
	}
	p.metaFields = fields

	// If the disk cache is approaching expiry, refresh in the background
	// so the next session has a warm cache.
	p.backgroundRefreshIfNeeded()

	return p, nil
}

// Search returns work items matching the named filter.
// By default, a fresh disk cache is returned without hitting the API.
// Pass noCache=true to force a fresh fetch.
func (p *Provider) Search(ctx context.Context, filter string, noCache bool) ([]*core.WorkItem, error) {
	// Try cache first unless caller explicitly wants fresh data.
	if !noCache && p.cacheDir != "" {
		if cached, err := loadCache(p.cacheDir, p.ws.Slug, filter, p.ws.CacheTTL); err == nil {
			return issuesToWorkItems(cached.Issues, p.wellKnown, p.customFieldMap()), nil
		}
	}

	jql, err := buildJQL(p.ws, p.cfg, filter)
	if err != nil {
		return nil, err
	}

	issues, err := fetchAllIssues(ctx, p.client, jql, p.cfg.FormattedFields, p.customFieldIDs())
	if err != nil {
		return nil, err
	}

	// Save to cache for future calls.
	if p.cacheDir != "" {
		_ = saveCache(p.cacheDir, p.ws.Slug, filter, issues)
	}

	return issuesToWorkItems(issues, p.wellKnown, p.customFieldMap()), nil
}

// Get returns a single work item by its Jira issue key.
func (p *Provider) Get(ctx context.Context, id string) (*core.WorkItem, error) {
	iss, err := p.client.FetchIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching issue %s: %w", id, err)
	}
	return issueToWorkItem(iss, p.wellKnown, p.customFieldMap()), nil
}

// Create persists a new work item and returns its assigned key.
func (p *Provider) Create(ctx context.Context, item *core.WorkItem) (string, error) {
	fields := map[string]any{
		"summary": item.Summary,
		"project": map[string]any{"key": p.cfg.ProjectKey},
	}

	for _, t := range p.ws.Types {
		if t.Name == item.Type {
			fields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", t.ID)}
			break
		}
	}

	if item.ParentID != "" {
		fields["parent"] = map[string]any{"key": strings.ToUpper(item.ParentID)}
	}

	if item.Description != nil {
		fields["description"] = renderADFValue(item.Description)
	}

	tx, err := p.wellKnown.TranslateFields(p, ctx, item.Fields)
	if err != nil {
		return "", err
	}

	maps.Copy(fields, tx.fields)

	created, err := p.client.CreateIssue(ctx, map[string]any{"fields": fields})
	if err != nil {
		return "", fmt.Errorf("creating issue: %w", err)
	}

	return created.Key, nil
}

// Update applies changes to an existing work item.
func (p *Provider) Update(ctx context.Context, id string, changes *core.Changes) error {
	fields := make(map[string]any)

	if changes.Summary != nil {
		fields["summary"] = *changes.Summary
	}

	if changes.Type != nil {
		for _, t := range p.ws.Types {
			if strings.EqualFold(t.Name, *changes.Type) {
				fields["issuetype"] = map[string]any{"id": fmt.Sprintf("%d", t.ID)}
				break
			}
		}
	}

	if changes.ParentID != nil {
		if *changes.ParentID == "" {
			fields["parent"] = nil // clear parent
		} else {
			fields["parent"] = map[string]any{"key": strings.ToUpper(*changes.ParentID)}
		}
	}

	if changes.Description != nil {
		fields["description"] = renderADFValue(changes.Description)
	}

	tx, err := p.wellKnown.TranslateFields(p, ctx, changes.Fields)
	if err != nil {
		return err
	}
	for k, v := range tx.fields {
		fields[k] = v
	}

	if len(fields) > 0 {
		if err := p.client.UpdateIssue(ctx, id, map[string]any{"fields": fields}); err != nil {
			return fmt.Errorf("updating issue %s: %w", id, err)
		}
	}

	if tx.assignUser != nil {
		if err := p.client.AssignIssue(ctx, id, *tx.assignUser); err != nil {
			return fmt.Errorf("assigning %s: %w", id, err)
		}
	}

	if changes.Status != nil {
		if err := performTransition(ctx, p.client, id, *changes.Status); err != nil {
			return fmt.Errorf("transitioning %s to '%s': %w", id, *changes.Status, err)
		}
	}

	if tx.sprintTarget != "" {
		if err := sprintAssign(ctx, p.client, p.cfg.BoardID, id, tx.sprintTarget); err != nil {
			return fmt.Errorf("assigning %s to %s sprint: %w", id, tx.sprintTarget, err)
		}
	}

	return nil
}

// translatedFields holds the result of translating alias-keyed field values
// into Jira API format, including side-effect actions that require separate
// API calls (assignee, sprint).
type translatedFields struct {
	fields       map[string]any // Jira field-key → API value
	sprintTarget string         // "active", "future", "none", or ""
	assignUser   *string        // accountId to assign; nil = no change, "" = unassign
}

// Comment adds a comment to a Jira issue.
func (p *Provider) Comment(ctx context.Context, id string, body string) error {
	ast, err := document.ParseMarkdownString(body)
	if err != nil {
		return fmt.Errorf("parsing comment: %w", err)
	}
	adfBody := renderADFValue(ast)
	return p.client.AddComment(ctx, id, adfBody)
}

// Assign assigns the issue to the current authenticated user.
func (p *Provider) Assign(ctx context.Context, id string) error {
	u, err := p.resolveUser(ctx)
	if err != nil {
		return fmt.Errorf("fetching current user: %w", err)
	}
	return p.client.AssignIssue(ctx, id, u.AccountID)
}

// CurrentUser returns the authenticated Jira user.
func (p *Provider) CurrentUser(ctx context.Context) (*core.User, error) {
	u, err := p.resolveUser(ctx)
	if err != nil {
		return nil, err
	}
	return &core.User{
		ID:          u.AccountID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
	}, nil
}

// resolveUser returns the cached user or fetches and caches it.
func (p *Provider) resolveUser(ctx context.Context) (*user, error) {
	if p.cachedUser != nil {
		return p.cachedUser, nil
	}
	u, err := p.client.FetchMyself(ctx)
	if err != nil {
		return nil, err
	}
	p.cachedUser = u
	return p.cachedUser, nil
}

// Capabilities returns the feature set supported by the Jira provider.
func (p *Provider) Capabilities() core.Capabilities {
	return core.Capabilities{
		HasHierarchy:   true,
		HasTransitions: true,
		HasTypes:       true,
		StatusSource:   core.StatusSourceWorkflow,
	}
}

// TransitionsFor returns the selectable workflow transition names for the
// issue along with its current status name. Jira filters transitions by
// workflow on the server, so we simply surface what the API returns.
func (p *Provider) TransitionsFor(ctx context.Context, id string) (string, []string, error) {
	item, err := p.Get(ctx, id)
	if err != nil {
		return "", nil, err
	}
	transitions, err := p.client.FetchTransitions(ctx, id)
	if err != nil {
		return "", nil, fmt.Errorf("fetching transitions for %s: %w", id, err)
	}
	opts := make([]string, 0, len(transitions))
	for _, t := range transitions {
		name := t.To.Name
		if name == "" {
			name = t.Name
		}
		// Skip transitions that land on the current status — they're no-ops.
		if strings.EqualFold(name, item.Status) {
			continue
		}
		opts = append(opts, name)
	}
	return item.Status, opts, nil
}

// ContentRenderer returns the Jira ADF content renderer.
func (p *Provider) ContentRenderer() core.ContentRenderer {
	return &adfRenderer{}
}

// FieldDefinitions returns the metadata describing Jira's fields.
// Field metadata is loaded eagerly at construction time.
func (p *Provider) FieldDefinitions() core.FieldDefs {
	return p.metaFields
}

// loadFieldMeta fetches createmeta data (disk cache → API), merges it with
// the global hardcoded fields, populates per-type FieldDefs on TypeConfig,
// and returns the union FieldDefs. Also builds the nameToID lookup table.
func (p *Provider) loadFieldMeta() (core.FieldDefs, error) {
	if p.cacheDir == "" || p.cfg == nil {
		return nil, fmt.Errorf("no cache dir or config")
	}

	meta, err := p.resolveCreateMeta()
	if err != nil {
		// TODO: structured/debug logging at this fallback point.
		return nil, err
	}

	globals := p.wellKnown.ToFieldDefs()
	p.nameToID = make(map[string]string)

	// Track all fields across types for the union set.
	seen := make(map[string]bool)    // key → added to extraDefs
	seenFID := make(map[string]bool) // fieldID → already in union (prevents same Jira field appearing twice)
	var extraDefs core.FieldDefs

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
					if !seen[def.Key] {
						seen[def.Key] = true
						seenFID[def.FieldID] = true
						extraDefs = append(extraDefs, def)
					}
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
					if !seen[def.Key] {
						seen[def.Key] = true
						seenFID[def.FieldID] = true
						extraDefs = append(extraDefs, def)
					}
				}
			}
		}

		// Add remaining non-global createmeta fields. Key is derived from
		// the Jira field name (e.g. "Epic Link" → "epic_link"). On collision
		// the numeric custom field ID is appended (e.g. "team_20001").
		// Skip any field whose FieldID is already in the union (via alias
		// or a previous type) to prevent the same Jira field appearing
		// under both its alias and an auto-derived key.
		for _, mf := range metaFields {
			if p.isExcludedField(mf.FieldID) || aliasedIDs[mf.FieldID] || seenFID[mf.FieldID] {
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
			if !seen[def.Key] {
				seen[def.Key] = true
				seenFID[mf.FieldID] = true
				extraDefs = append(extraDefs, def)
			}
		}

		tc.Fields = typeDefs
	}

	// Union: globals + all extra fields discovered across types.
	union := make(core.FieldDefs, len(globals), len(globals)+len(extraDefs))
	copy(union, globals)

	// Patch global enum values from the first type that has them.
	if len(p.ws.Types) > 0 {
		for i := range p.ws.Types {
			tc := &p.ws.Types[i]
			typeID := fmt.Sprintf("%d", tc.ID)
			if metaFields, ok := meta.Types[typeID]; ok {
				metaByID := make(map[string]createMetaField, len(metaFields))
				for _, mf := range metaFields {
					metaByID[mf.FieldID] = mf
				}
				p.linkGlobalsToMeta(union, metaByID)
				break
			}
		}
	}

	union = append(union, extraDefs...)
	return union, nil
}

// backgroundRefreshIfNeeded checks the disk cache age and triggers a
// background API refresh if it's past MetaCacheRefreshThreshold of the TTL.
// This keeps the cache warm so the next session doesn't hit the API
// synchronously (which causes UI pop-in).
func (p *Provider) backgroundRefreshIfNeeded() {
	if p.cacheDir == "" || p.cfg == nil {
		return
	}
	path := createMetaCachePath(p.cacheDir, p.ws.ServerAlias, p.cfg.ProjectKey)
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
			ServerAlias: p.ws.ServerAlias,
			ProjectKey:  p.cfg.ProjectKey,
			Types:       make(map[string][]createMetaField),
		}
		for _, tc := range p.ws.Types {
			typeID := fmt.Sprintf("%d", tc.ID)
			fields, err := p.client.FetchCreateMetaFields(ctx, p.cfg.ProjectKey, typeID)
			if err != nil {
				return // silently abandon — current cache is still valid
			}
			meta.Types[typeID] = fields
		}
		_ = saveCreateMetaCache(p.cacheDir, p.ws.ServerAlias, p.cfg.ProjectKey, meta)
	}()
}

// resolveCreateMeta loads createmeta from disk cache or fetches from the API.
func (p *Provider) resolveCreateMeta() (*cachedCreateMeta, error) {
	alias := p.ws.ServerAlias
	project := p.cfg.ProjectKey

	// Try disk cache first.
	if cached, err := loadCreateMetaCache(p.cacheDir, alias, project, DefaultMetaCacheTTL); err == nil {
		return cached, nil
	}

	// No cache — fetch from API. Print a status line so the user knows
	// why there's a brief pause on first run.
	fmt.Fprintf(os.Stderr, "Loading field metadata for %s…\n", project)

	ctx := context.Background()
	meta := &cachedCreateMeta{
		ServerAlias: alias,
		ProjectKey:  project,
		Types:       make(map[string][]createMetaField),
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
	_ = saveCreateMetaCache(p.cacheDir, alias, project, meta)
	return meta, nil
}

// linkGlobalsToMeta populates well-known global FieldDefs with runtime data
// from createmeta: priority enum values + nameToID lookup, sprint FieldID,
// team FieldID. Called per-type to build type-specific copies and once on
// the union for the provider-wide FieldDefinitions.
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

// metaFieldToDef converts a createmeta field into a core.FieldDef.
// knownFieldIcons maps well-known custom field aliases to icons.
var knownFieldIcons = map[string]string{
	"story_points": core.IconStoryPoints,
	"sprint":       core.IconSprint,
	"team":         core.IconTeam,
}

// nameToKey derives a snake_case key from a Jira field name.
// e.g. "Epic Link" → "epic_link", "Story Points" → "story_points".
func nameToKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r == ' ' || r == '-':
			b.WriteByte('_')
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func metaFieldToDef(mf createMetaField, pinned bool) core.FieldDef {
	def := core.FieldDef{
		Key:      mf.Key,
		Label:    mf.Name,
		FieldID:  mf.FieldID,
		Required: mf.Required,
		Pinned:   pinned,
		Role:     core.RoleCustom,
		Type:     schemaToFieldType(mf.Schema),
	}

	if icon, ok := knownFieldIcons[mf.Key]; ok {
		def.Icon = icon
	}

	if len(mf.AllowedValues) > 0 {
		names, _ := extractAllowedValues(mf.AllowedValues)
		if len(names) > 0 {
			def.Type = core.FieldEnum
			def.Enum = names
		}
	}

	return def
}

// knownCustomTypes maps Jira custom field plugin types to their core
// FieldType. Only fields whose Custom value appears here (or whose Custom
// is empty — meaning a system field) are included in FieldDefs. Unknown
// plugin types are excluded by default to avoid surfacing internal or
// display-only fields. Add new entries here as needed.
var knownCustomTypes = map[string]core.FieldType{
	// --- Built-in (com.atlassian.jira.plugin.system.customfieldtypes) ---
	"com.atlassian.jira.plugin.system.customfieldtypes:textfield":        core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:textarea":         core.FieldRichText, // ADF in v3
	"com.atlassian.jira.plugin.system.customfieldtypes:float":            core.FieldString,   // number as string
	"com.atlassian.jira.plugin.system.customfieldtypes:datepicker":       core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:datetime":         core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:url":              core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:select":           core.FieldEnum,
	"com.atlassian.jira.plugin.system.customfieldtypes:radiobuttons":     core.FieldEnum,
	"com.atlassian.jira.plugin.system.customfieldtypes:multiselect":      core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:multicheckboxes":  core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:userpicker":       core.FieldAssignee,
	"com.atlassian.jira.plugin.system.customfieldtypes:multiuserpicker":  core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:grouppicker":      core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:multigrouppicker": core.FieldStringArray,
	"com.atlassian.jira.plugin.system.customfieldtypes:project":          core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:version":          core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:atlassian-team":   core.FieldString,
	"com.atlassian.jira.plugin.system.customfieldtypes:labels":           core.FieldStringArray,

	// --- Greenhopper / Jira Software ---
	"com.pyxis.greenhopper.jira:gh-sprint":    core.FieldEnum, // sprint picker
	"com.pyxis.greenhopper.jira:gh-epic-link": core.FieldString,

	// --- Tempo ---
	"com.tempoplugin.tempo-accounts:accounts.customfield": core.FieldString,
}

// isKnownCustomType reports whether a createmeta field's Custom value is
// one we know how to handle. System fields (Custom == "") are always known.
func isKnownCustomType(custom string) bool {
	if custom == "" {
		return true // system field, no plugin type
	}
	_, ok := knownCustomTypes[custom]
	return ok
}

// schemaToFieldType maps a Jira field schema to a core.FieldType.
// For known custom types the mapping comes from knownCustomTypes; for
// system fields (Custom == "") the schema.Type drives the mapping.
func schemaToFieldType(s fieldSchema) core.FieldType {
	// Check custom type first — it's more specific than schema.Type.
	if ft, ok := knownCustomTypes[s.Custom]; ok {
		return ft
	}
	// System fields (no Custom value) fall through to schema.Type.
	switch s.Type {
	case "string":
		return core.FieldString
	case "number", "integer":
		return core.FieldString // numbers represented as strings in manifests
	case "array":
		return core.FieldStringArray
	case "option", "priority":
		return core.FieldEnum
	default:
		return core.FieldString
	}
}

// extractAllowedValues parses a JSON allowedValues array and returns
// parallel slices of display names and IDs.
func extractAllowedValues(raw json.RawMessage) (names []string, ids []string) {
	var values []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, nil
	}
	for _, v := range values {
		name := v.Name
		if name == "" {
			name = v.Value
		}
		if name == "" {
			continue
		}
		names = append(names, name)
		ids = append(ids, v.ID)
	}
	return names, ids
}

// customFieldIDs returns the Jira field IDs (e.g. "customfield_10016") for
// all dynamic fields discovered via createmeta. Collects from per-type
// FieldDefs (not the union) so that different types mapping different field
// IDs to the same alias all get requested.
func (p *Provider) customFieldIDs() []string {
	_ = p.FieldDefinitions() // ensure createmeta is loaded
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
// dynamic fields. Collects from per-type FieldDefs so that different types
// mapping different field IDs to the same alias all get extracted correctly.
func (p *Provider) customFieldMap() map[string]customFieldBinding {
	_ = p.FieldDefinitions() // ensure createmeta is loaded
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

// priorityPayload returns the Jira API payload for a priority value.
// Uses the nameToID lookup (populated from createmeta) when available,
// falling back to name-based matching.
func (p *Provider) priorityPayload(name string) map[string]any {
	if p.nameToID != nil {
		if id, ok := p.nameToID["priority:"+name]; ok {
			return map[string]any{"id": id}
		}
	}
	return map[string]any{"name": name}
}

// resolveEmailToAccountID looks up a Jira user by email and returns their accountId.
func (p *Provider) resolveEmailToAccountID(ctx context.Context, email string) (string, error) {
	users, err := p.client.SearchUsers(ctx, email)
	if err != nil {
		return "", fmt.Errorf("searching users for %q: %w", email, err)
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u.AccountID, nil
		}
	}
	if len(users) > 0 {
		return users[0].AccountID, nil
	}
	return "", fmt.Errorf("no user found for email %q", email)
}

// adfRenderer implements core.ContentRenderer for Jira's ADF format.
type adfRenderer struct{}

func (r *adfRenderer) ParseContent(raw any) (*document.Node, error) {
	switch v := raw.(type) {
	case json.RawMessage:
		return parseADF(v)
	case []byte:
		return parseADF(v)
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshaling ADF: %w", err)
		}
		return parseADF(data)
	default:
		return nil, fmt.Errorf("unsupported ADF input type: %T", raw)
	}
}

func (r *adfRenderer) RenderContent(node *document.Node) (any, error) {
	return renderADFValue(node), nil
}
