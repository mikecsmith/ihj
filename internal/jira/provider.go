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
	"strings"
	"sync"

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

	// metaOnce guards lazy loading of createmeta field metadata.
	metaOnce   sync.Once
	metaFields core.FieldDefs    // union of all type fields (populated by ensureCreateMeta)
	metaErr    error             // non-nil if createmeta load failed
	nameToID   map[string]string // "fieldKey:valueName" → "valueID" for payload construction
}

// Compile-time check that *Provider implements core.Provider.
var _ core.Provider = (*Provider)(nil)

// NewProvider creates a Jira provider for the given workspace.
// The workspace's ProviderConfig must already be a *jira.Config
// (hydrated by the composition root).
// cacheDir may be empty to disable disk caching.
func NewProvider(client API, ws *core.Workspace, cacheDir string) *Provider {
	cfg, _ := ws.ProviderConfig.(*Config)
	return &Provider{
		client:   client,
		ws:       ws,
		cfg:      cfg,
		cacheDir: cacheDir,
	}
}

// Search returns work items matching the named filter.
// By default, a fresh disk cache is returned without hitting the API.
// Pass noCache=true to force a fresh fetch.
func (p *Provider) Search(ctx context.Context, filter string, noCache bool) ([]*core.WorkItem, error) {
	// Try cache first unless caller explicitly wants fresh data.
	if !noCache && p.cacheDir != "" {
		if cached, err := loadCache(p.cacheDir, p.ws.Slug, filter, p.ws.CacheTTL); err == nil {
			return issuesToWorkItems(cached.Issues, p.customFieldMap()), nil
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

	return issuesToWorkItems(issues, p.customFieldMap()), nil
}

// Get returns a single work item by its Jira issue key.
func (p *Provider) Get(ctx context.Context, id string) (*core.WorkItem, error) {
	iss, err := p.client.FetchIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching issue %s: %w", id, err)
	}
	return issueToWorkItem(iss, p.customFieldMap()), nil
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

	tx, err := p.translateFields(ctx, item.Fields)
	if err != nil {
		return "", err
	}
	for k, v := range tx.fields {
		fields[k] = v
	}

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
		fields["parent"] = map[string]any{"key": *changes.ParentID}
	}

	if changes.Description != nil {
		fields["description"] = renderADFValue(changes.Description)
	}

	tx, err := p.translateFields(ctx, changes.Fields)
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

// translateFields converts a map of alias-keyed field values (from
// Changes.Fields or WorkItem.Fields) into Jira API format. Handles
// alias→FieldID translation, RichText→ADF conversion, priority ID
// lookup, email→accountId resolution, and component/label formatting.
func (p *Provider) translateFields(ctx context.Context, src map[string]any) (*translatedFields, error) {
	tx := &translatedFields{fields: make(map[string]any)}
	if len(src) == 0 {
		return tx, nil
	}

	defs := p.FieldDefinitions()
	defByKey := make(map[string]core.FieldDef, len(defs))
	for _, d := range defs {
		defByKey[d.Key] = d
	}

	for k, v := range src {
		switch k {
		case "sprint":
			if s, ok := v.(string); ok && (s == "active" || s == "future" || s == "none") {
				tx.sprintTarget = s
			}
		case "priority":
			if s, ok := v.(string); ok && s != "" {
				tx.fields["priority"] = p.priorityPayload(s)
			}
		case "assignee":
			if email, ok := v.(string); ok {
				if email == "" {
					empty := ""
					tx.assignUser = &empty
				} else {
					accountID, err := p.resolveEmailToAccountID(ctx, email)
					if err != nil {
						return nil, fmt.Errorf("resolving assignee %q: %w", email, err)
					}
					tx.assignUser = &accountID
				}
			}
		case "reporter":
			if email, ok := v.(string); ok && email != "" {
				accountID, err := p.resolveEmailToAccountID(ctx, email)
				if err != nil {
					return nil, fmt.Errorf("resolving reporter %q: %w", email, err)
				}
				tx.fields["reporter"] = map[string]any{"accountId": accountID}
			}
		case "labels":
			if labels, ok := v.([]string); ok {
				tx.fields["labels"] = labels
			}
		case "components":
			if comps, ok := v.([]string); ok {
				jiraComps := make([]map[string]any, len(comps))
				for i, c := range comps {
					jiraComps[i] = map[string]any{"name": c}
				}
				tx.fields["components"] = jiraComps
			}
		case "team":
			// Boolean action field: true → set team UUID, false → clear.
			// Omit is handled by absence from src.
			if p.cfg.TeamUUID == "" {
				continue
			}
			def := defByKey[k]
			jiraKey := k
			if def.FieldID != "" && !isGlobalField(def.FieldID) {
				jiraKey = def.FieldID
			}
			switch val := v.(type) {
			case bool:
				if val {
					tx.fields[jiraKey] = p.cfg.TeamUUID
				} else {
					tx.fields[jiraKey] = nil
				}
			case string:
				if strings.EqualFold(val, "true") {
					tx.fields[jiraKey] = p.cfg.TeamUUID
				} else if strings.EqualFold(val, "false") {
					tx.fields[jiraKey] = nil
				}
			}
		default:
			def := defByKey[k]
			if def.Type == core.FieldRichText {
				if node, ok := v.(*document.Node); ok {
					v = renderADFValue(node)
				}
			}
			jiraKey := k
			if def.FieldID != "" && !isGlobalField(def.FieldID) {
				jiraKey = def.FieldID
			}
			tx.fields[jiraKey] = v
		}
	}
	return tx, nil
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
// On first call it lazily loads createmeta data (from disk cache or API),
// merges dynamic enums and custom fields with the hardcoded globals,
// and populates per-type FieldDefs on TypeConfig. Falls back to the
// hardcoded definitions if createmeta is unavailable (e.g. 403/404).
func (p *Provider) FieldDefinitions() core.FieldDefs {
	p.metaOnce.Do(func() {
		p.metaFields, p.metaErr = p.loadFieldMeta()
	})
	if p.metaErr != nil || p.metaFields == nil {
		return p.hardcodedFieldDefs()
	}
	return p.metaFields
}

// hardcodedFieldDefs returns the static field definitions used when
// createmeta data is unavailable. Priority enum is a best-guess default.
func (p *Provider) hardcodedFieldDefs() core.FieldDefs {
	defs := core.FieldDefs{
		{Key: "priority", Label: "Priority", Short: "P", Type: core.FieldEnum,
			Enum: []string{"Highest", "High", "Medium", "Low", "Lowest"},
			Role: core.RoleUrgency, Primary: true},
		{Key: "assignee", Label: "Assignee", Icon: core.IconUser, Type: core.FieldAssignee,
			Role: core.RoleOwnership, Primary: true},
		{Key: "labels", Label: "Labels", Icon: core.IconTag, Type: core.FieldStringArray,
			Role: core.RoleCategorisation, Primary: true},
		{Key: "components", Label: "Components", Icon: core.IconCube, Type: core.FieldStringArray,
			Role: core.RoleCategorisation},
	}

	if p.cfg.BoardType == "scrum" {
		defs = append(defs, core.FieldDef{
			Key: "sprint", Label: "Sprint", Type: core.FieldEnum,
			Enum: []string{"active", "future", "none"},
			Role: core.RoleIteration, Primary: true,
			WriteOnly: true,
		})
	}

	defs = append(defs,
		core.FieldDef{Key: "reporter", Label: "Reporter", Icon: core.IconUserCard, Type: core.FieldEmail,
			Role: core.RoleOwnership},
		core.FieldDef{Key: "created", Label: "Created", Icon: core.IconCalendar, Type: core.FieldString,
			Role: core.RoleTemporal, Primary: true, Derived: true, Immutable: true},
		core.FieldDef{Key: "updated", Label: "Updated", Icon: core.IconRefresh, Type: core.FieldString,
			Role: core.RoleTemporal, Derived: true, Immutable: true},
	)

	return defs
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

	globals := p.hardcodedFieldDefs()
	p.nameToID = make(map[string]string)

	// Track all fields across types for the union set.
	seen := make(map[string]bool)
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
		p.patchGlobalsFromMeta(typeDefs, metaByID)

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
					typeDefs = append(typeDefs, def)
					if !seen[def.Key] {
						seen[def.Key] = true
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
					typeDefs = append(typeDefs, def)
					if !seen[def.Key] {
						seen[def.Key] = true
						extraDefs = append(extraDefs, def)
					}
				}
			}
		}

		// Add remaining non-global createmeta fields that aren't aliased.
		// These appear in schemas so users can set fields they haven't
		// explicitly configured in the workspace. Key is derived from the
		// Jira field name (e.g. "Epic Link" → "epic_link"). On collision
		// the numeric custom field ID is appended (e.g. "team_20001").
		for _, mf := range metaFields {
			if isGlobalField(mf.FieldID) || aliasedIDs[mf.FieldID] {
				continue
			}
			def := metaFieldToDef(mf, false)
			if key := nameToKey(mf.Name); key != "" {
				def.Key = key
			}
			if typeDefs.WithKey(def.Key) != nil {
				def.Key = def.Key + "_" + strings.TrimPrefix(mf.FieldID, "customfield_")
			}
			if typeDefs.WithKey(def.Key) != nil {
				continue // still collides (shouldn't happen) — skip
			}
			typeDefs = append(typeDefs, def)
			if !seen[def.Key] {
				seen[def.Key] = true
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
				p.patchGlobalsFromMeta(union, metaByID)
				break
			}
		}
	}

	union = append(union, extraDefs...)
	return union, nil
}

// resolveCreateMeta loads createmeta from disk cache or fetches from the API.
func (p *Provider) resolveCreateMeta() (*cachedCreateMeta, error) {
	alias := p.ws.ServerAlias
	project := p.cfg.ProjectKey

	// Try disk cache first.
	if cached, err := loadCreateMetaCache(p.cacheDir, alias, project, DefaultMetaCacheTTL); err == nil {
		return cached, nil
	}

	// Fetch from API for each configured type.
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

// patchGlobalsFromMeta updates global fields using createmeta data:
// patches priority enum values and links the sprint field to its Jira field ID.
func (p *Provider) patchGlobalsFromMeta(defs core.FieldDefs, metaByID map[string]createMetaField) {
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
		}
	}
}

// isGlobalField returns true if the field ID corresponds to a built-in
// global field that's already in hardcodedFieldDefs.
func isGlobalField(fieldID string) bool {
	switch fieldID {
	case "priority", "assignee", "labels", "components", "reporter",
		"created", "updated", "summary", "description", "issuetype",
		"status", "parent", "comment", "project":
		return true
	}
	return false
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

	// Well-known action fields: boolean write-only semantics regardless
	// of what the Jira schema reports.
	if mf.Key == "team" {
		def.Type = core.FieldBool
		def.WriteOnly = true
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

// schemaToFieldType maps a Jira field schema to a core.FieldType.
func schemaToFieldType(s fieldSchema) core.FieldType {
	// Multi-line text custom fields return ADF content in v3, so we
	// classify them as rich text regardless of the top-level type.
	if s.Custom == "com.atlassian.jira.plugin.system.customfieldtypes:textarea" {
		return core.FieldRichText
	}
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
			if d.FieldID != "" && !isGlobalField(d.FieldID) && !seen[d.FieldID] {
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
			if d.FieldID != "" && !isGlobalField(d.FieldID) {
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
