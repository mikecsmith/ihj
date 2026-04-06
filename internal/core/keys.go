package core

// Reserved key constants used across encoding surfaces (manifest, frontmatter,
// schema). Using consts prevents typo-driven bugs and enables refactoring.
const (
	// Identity.
	KeyKey = "key"

	// Content (struct-backed on WorkItem).
	KeySummary     = "summary"
	KeyType        = "type"
	KeyStatus      = "status"
	KeyParent      = "parent"
	KeyDescription = "description"

	// Structural containers.
	KeyChildren = "children"
	KeyComments = "comments"
	KeyFields   = "fields"
)

// CoreFields lists the struct-backed content keys of a WorkItem in canonical
// display order. These keys map to dedicated fields on WorkItem (Summary, Type,
// Status, ParentID, Description) — never into the Fields bag.
var CoreFields = []string{KeySummary, KeyType, KeyStatus, KeyParent, KeyDescription}

// reservedKeys is the union of CoreFields plus identity and structural keys
// that must not be routed into the Fields bag during decode, regardless of
// encoding surface (manifest or frontmatter).
var reservedKeys = map[string]bool{
	KeyKey:         true,
	KeySummary:     true,
	KeyType:        true,
	KeyStatus:      true,
	KeyParent:      true,
	KeyDescription: true,
	KeyChildren:    true,
	KeyComments:    true,
	KeyFields:      true,
}

// coreFieldSet mirrors CoreFields for O(1) membership tests.
var coreFieldSet = func() map[string]bool {
	m := make(map[string]bool, len(CoreFields))
	for _, k := range CoreFields {
		m[k] = true
	}
	return m
}()

// IsCoreField reports whether a key maps to a struct-backed content field on
// WorkItem (summary / type / status / parent / description). Parsers use this
// to route known keys to struct properties before falling through to the
// Fields bag.
func IsCoreField(k string) bool { return coreFieldSet[k] }

// IsReservedKey reports whether a key is reserved by the schema — either a
// core content field, the identity key, or a structural container (children,
// comments, fields). Reserved keys must never appear in WorkItem.Fields.
func IsReservedKey(k string) bool { return reservedKeys[k] }
