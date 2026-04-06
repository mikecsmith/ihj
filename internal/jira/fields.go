package jira

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// wellKnownField holds metadata for a field the Jira provider recognises.
// This is the single source of truth for which fields are well-known and
// how they behave across the provider's pathways (search, exclusion,
// FieldDef generation, extraction). Existing code paths consult this map
// instead of hardcoding field names in multiple locations.
type wellKnownField struct {
	Key       string         // alias key (e.g. "priority", "sprint")
	Label     string         // human-readable name
	Short     string         // abbreviated label (e.g. "P" for priority)
	Icon      string         // Nerd Font icon
	FieldType core.FieldType // data type (empty for system-only fields that don't surface as FieldDefs)
	Role      core.FieldRole // semantic grouping

	// Behavioural flags — drive FieldDef generation.
	Primary   bool // prominent placement
	Derived   bool // system-computed, not user-writable
	Immutable bool // set once, never changes
	WriteOnly bool // action field — fire-and-forget command, not state
	Required  bool // required for creation

	// Pathway flags — which parts of the flow care about this field.
	SearchField bool // include in search API field list
	Excluded    bool // exclude from createmeta FieldDef discovery
}

// wellKnownFields is a named map type with methods for deriving
// the data structures that each consumer needs.
type wellKnownFields map[string]wellKnownField

// knownJSONKeys is the set of field keys excluded from the Customs map
// during issueFields.UnmarshalJSON. Derived from the well-known field registry.
var knownJSONKeys = defaultWellKnownFields().KnownJSONKeys()

// defaultWellKnownFields returns the static well-known fields without any
// config-dependent entries (sprint, team). Used for package-level derivations
// like StandardFields that are needed before a Provider is initialised.
func defaultWellKnownFields() wellKnownFields {
	p := &Provider{} // nil cfg → config-dependent entries skipped
	return p.buildWellKnownFields()
}

// buildWellKnownFields constructs the provider's well-known field registry.
// Method because some entries depend on workspace config (board type, team UUID).
func (p *Provider) buildWellKnownFields() wellKnownFields {
	wk := wellKnownFields{
		// --- Standard fields with FieldDefs ---
		"priority": {
			Key: "priority", Label: "Priority", Short: "P",
			FieldType: core.FieldEnum, Role: core.RoleUrgency,
			Primary: true, SearchField: true, Excluded: true,
		},
		"assignee": {
			Key: "assignee", Label: "Assignee", Icon: core.IconUser,
			FieldType: core.FieldAssignee, Role: core.RoleOwnership,
			Primary: true, SearchField: true, Excluded: true,
		},
		"labels": {
			Key: "labels", Label: "Labels", Icon: core.IconTag,
			FieldType: core.FieldStringArray, Role: core.RoleCategorisation,
			Primary: true, SearchField: true, Excluded: true,
		},
		"components": {
			Key: "components", Label: "Components", Icon: core.IconCube,
			FieldType: core.FieldStringArray, Role: core.RoleCategorisation,
			SearchField: true, Excluded: true,
		},
		"reporter": {
			Key: "reporter", Label: "Reporter", Icon: core.IconUserCard,
			FieldType: core.FieldEmail, Role: core.RoleOwnership,
			SearchField: true, Excluded: true,
		},
		"created": {
			Key: "created", Label: "Created", Icon: core.IconCalendar,
			FieldType: core.FieldString, Role: core.RoleTemporal,
			Primary: true, Derived: true, Immutable: true,
			SearchField: true, Excluded: true,
		},
		"updated": {
			Key: "updated", Label: "Updated", Icon: core.IconRefresh,
			FieldType: core.FieldString, Role: core.RoleTemporal,
			Derived: true, Immutable: true,
			SearchField: true, Excluded: true,
		},

		// --- Core structural fields (no FieldDef — handled by WorkItem struct) ---
		"summary":     {Key: "summary", SearchField: true, Excluded: true},
		"description": {Key: "description", SearchField: true, Excluded: true},
		"issuetype":   {Key: "issuetype", SearchField: true, Excluded: true},
		"status":      {Key: "status", SearchField: true, Excluded: true},
		"parent":      {Key: "parent", SearchField: true, Excluded: true},
		"project":     {Key: "project", Excluded: true},
		"comment":     {Key: "comment", SearchField: true, Excluded: true},
		"subtasks":    {Key: "subtasks", SearchField: true, Excluded: true},

		// --- System metadata (excluded from FieldDef discovery) ---
		"creator":                       {Key: "creator", Excluded: true},
		"issuelinks":                    {Key: "issuelinks", Excluded: true},
		"attachment":                    {Key: "attachment", Excluded: true},
		"worklog":                       {Key: "worklog", Excluded: true},
		"votes":                         {Key: "votes", Excluded: true},
		"watches":                       {Key: "watches", Excluded: true},
		"thumbnail":                     {Key: "thumbnail", Excluded: true},
		"timetracking":                  {Key: "timetracking", Excluded: true},
		"aggregatetimeoriginalestimate": {Key: "aggregatetimeoriginalestimate", Excluded: true},
		"aggregatetimespent":            {Key: "aggregatetimespent", Excluded: true},
		"aggregateprogress":             {Key: "aggregateprogress", Excluded: true},
		"progress":                      {Key: "progress", Excluded: true},
		"timeoriginalestimate":          {Key: "timeoriginalestimate", Excluded: true},
		"timespent":                     {Key: "timespent", Excluded: true},
		"environment":                   {Key: "environment", Excluded: true},
		"duedate":                       {Key: "duedate", Excluded: true},
		"resolution":                    {Key: "resolution", Excluded: true},
		"resolutiondate":                {Key: "resolutiondate", Excluded: true},
		"fixVersions":                   {Key: "fixVersions", Excluded: true},
		"versions":                      {Key: "versions", Excluded: true},
		"security":                      {Key: "security", Excluded: true},
		"lastViewed":                    {Key: "lastViewed", Excluded: true},
		"workratio":                     {Key: "workratio", Excluded: true},
		"statuscategorychangedate":      {Key: "statuscategorychangedate", Excluded: true},
	}

	// Conditional entries based on workspace config.
	// Sprint is a custom field in Jira (customfield_XXXXX) but we treat it
	// as a well-known field for FieldDef generation. NOT Excluded or SearchField
	// because those operate on Jira field IDs, and sprint's Jira ID is a
	// custom field ID that arrives via extraFields/customFieldBinding.
	if p.cfg != nil && p.cfg.BoardType == "scrum" {
		wk["sprint"] = wellKnownField{
			Key: "sprint", Label: "Sprint",
			FieldType: core.FieldEnum, Role: core.RoleIteration,
			Primary: true, WriteOnly: true,
		}
	}
	// Team is a custom field in Jira but treated as a well-known action field.
	// NOT Excluded or SearchField — same reasoning as sprint.
	if p.cfg != nil && p.cfg.TeamUUID != "" {
		wk["team"] = wellKnownField{
			Key: "team", Label: "Team", Icon: core.IconTeam,
			FieldType: core.FieldBool, Role: core.RoleCustom,
			Primary: true, WriteOnly: true,
		}
	}

	return wk
}

// ToFieldDefs converts well-known fields into core.FieldDefs. Only entries with a non-empty
// FieldType produce a FieldDef. Sorted by key for stability.
func (wk wellKnownFields) ToFieldDefs() core.FieldDefs {
	var defs core.FieldDefs
	for _, f := range wk {
		if f.FieldType == "" {
			continue
		}
		def := core.FieldDef{
			Key:       f.Key,
			Label:     f.Label,
			Short:     f.Short,
			Icon:      f.Icon,
			Type:      f.FieldType,
			Role:      f.Role,
			Primary:   f.Primary,
			Derived:   f.Derived,
			Immutable: f.Immutable,
			WriteOnly: f.WriteOnly,
			Required:  f.Required,
		}
		// Hardcoded enum values for action fields whose allowed values
		// are semantic commands, not dynamic API data. Regular enum fields
		// (like priority) get their values from createmeta at runtime.
		if f.Key == "sprint" {
			def.Enum = []string{"active", "future", "none"}
		}
		defs = append(defs, def)
	}

	// Sort by a canonical order for stability: urgency, ownership,
	// categorisation, iteration, temporal, then custom — matching the
	// original declaration order.
	order := map[core.FieldRole]int{
		core.RoleUrgency:        0,
		core.RoleOwnership:      1,
		core.RoleCategorisation: 2,
		core.RoleIteration:      3,
		core.RoleTemporal:       4,
		core.RoleCustom:         5,
		core.RoleDefault:        6,
	}
	slices.SortFunc(defs, func(a, b core.FieldDef) int {
		oa, ob := order[a.Role], order[b.Role]
		if oa != ob {
			return oa - ob
		}
		return strings.Compare(a.Key, b.Key)
	})

	return defs
}

// ExcludedIDs returns the set of field IDs that should be excluded from
// createmeta FieldDef discovery. Replaces the hardcoded excludedFields map.
func (wk wellKnownFields) ExcludedIDs() map[string]bool {
	m := make(map[string]bool, len(wk))
	for k, f := range wk {
		if f.Excluded {
			m[k] = true
		}
	}
	return m
}

// SearchFields returns the field IDs to request in Jira search API calls.
// Replaces the hardcoded StandardFields slice.
func (wk wellKnownFields) SearchFields() []string {
	var fields []string
	for k, f := range wk {
		if f.SearchField {
			fields = append(fields, k)
		}
	}
	slices.Sort(fields)
	return fields
}

// KnownJSONKeys returns the set of field keys that have struct-backed
// fields on issueFields and should not be captured in the Customs map
// during UnmarshalJSON. This is the subset of well-known fields that
// the JSON deserializer handles via named struct fields.
func (wk wellKnownFields) KnownJSONKeys() map[string]bool {
	// Only fields that have struct backing on issueFields. These are the
	// core structural fields plus the standard fields with dedicated
	// struct fields (priority, assignee, reporter, labels, components, etc.).
	// System noise fields (votes, watches, etc.) do NOT have struct backing
	// and should be captured in Customs so they're silently ignored.
	structBacked := map[string]bool{
		"summary": true, "description": true, "issuetype": true,
		"status": true, "priority": true, "assignee": true,
		"reporter": true, "parent": true, "labels": true,
		"components": true, "comment": true, "created": true,
		"updated": true, "subtasks": true, "customs": true,
	}
	return structBacked
}

// ApplyOverrides patches a FieldDef with well-known field metadata if the
// key matches a well-known action field (e.g. team, sprint). This ensures
// aliased custom fields get the correct type and behavioural flags regardless
// of what createmeta reports.
func (wk wellKnownFields) ApplyOverrides(def *core.FieldDef) {
	f, ok := wk[def.Key]
	if !ok || f.FieldType == "" {
		return
	}
	// Only override action fields — regular well-known fields (priority,
	// assignee, etc.) get their metadata from the hardcoded FieldDef.
	if !f.WriteOnly {
		return
	}
	def.Type = f.FieldType
	def.WriteOnly = f.WriteOnly
	def.Primary = f.Primary
	if f.Icon != "" {
		def.Icon = f.Icon
	}
}

// translatedCases lists the field keys that have explicit translation logic
// in TranslateFields. This must stay in sync with the switch cases below.
// The init() check in fields_test.go verifies each case is a registered
// well-known field.
var translatedCases = []string{
	"sprint", "priority", "assignee", "reporter", "labels", "components", "team",
}

// TranslateFields converts alias-keyed field values into Jira API format.
// Well-known fields get per-field translation (priority ID lookup, email→accountId,
// component formatting, action field routing). Unknown fields get generic
// FieldID lookup and RichText→ADF conversion.
func (wk wellKnownFields) TranslateFields(p *Provider, ctx context.Context, src map[string]any) (*translatedFields, error) {
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
			if p.cfg.TeamUUID == "" {
				continue
			}
			def := defByKey[k]
			jiraKey := k
			if def.FieldID != "" && !p.isExcludedField(def.FieldID) {
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
			// Generic handler for unknown/custom fields: resolve Jira key
			// and convert RichText to ADF.
			def := defByKey[k]
			if def.Type == core.FieldRichText {
				if node, ok := v.(*document.Node); ok {
					v = renderADFValue(node)
				}
			}
			jiraKey := k
			if def.FieldID != "" && !p.isExcludedField(def.FieldID) {
				jiraKey = def.FieldID
			}
			tx.fields[jiraKey] = v
		}
	}
	return tx, nil
}

// ExtractFields extracts well-known field values from a Jira issue response
// into maps suitable for WorkItem.Fields and WorkItem.DisplayFields.
// Custom fields are handled separately via customFieldBinding.
func (wk wellKnownFields) ExtractFields(f *issueFields) (fields map[string]any, display map[string]any) {
	fields = make(map[string]any)
	display = make(map[string]any)

	for _, wf := range wk {
		if wf.FieldType == "" {
			continue // system-only field, no extraction
		}

		switch wf.Key {
		case "priority":
			fields["priority"] = f.Priority.Name
		case "assignee":
			fields["assignee"] = f.Assignee.EmailOrDefault("")
			display["assignee"] = f.Assignee.DisplayNameOrDefault("")
		case "reporter":
			fields["reporter"] = f.Reporter.EmailOrDefault("")
			display["reporter"] = f.Reporter.DisplayNameOrDefault("")
		case "labels":
			if len(f.Labels) > 0 {
				fields["labels"] = f.Labels
			}
		case "components":
			if len(f.Components) > 0 {
				var names []string
				for _, c := range f.Components {
					names = append(names, c.Name)
				}
				fields["components"] = names
			}
		case "created":
			fields["created"] = formatDate(f.Created)
			display["created"] = formatDisplayDate(f.Created)
		case "updated":
			fields["updated"] = formatDate(f.Updated)
			display["updated"] = formatDisplayDate(f.Updated)
		}
		// sprint and team are handled via customFieldBinding (they're custom fields in Jira)
	}

	return fields, display
}
