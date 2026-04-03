package core

// Icons used across the TUI. Nerd Font codepoints and standard Unicode
// glyphs are centralised here so individual source files stay ASCII-clean.
const (
	// Nerd Font — field labels. Each includes a trailing space.
	IconUser     = "\uf007 "     // nf-fa-user
	IconUserCard = "\uf2bd "     // nf-fa-address_card
	IconCalendar = "\uf073 "     // nf-fa-calendar
	IconRefresh  = "\uf021 "     // nf-fa-refresh
	IconTag      = "\uf02b "     // nf-fa-tag
	IconCube     = "\uf1b2 "     // nf-fa-cube
	IconTeam     = "\uf0c0 "     // nf-fa-users
	IconType     = "\ueb2b "     // nf-cod-symbol_class
	IconParent   = "\U000f0136 " // nf-md-link_variant
	IconChildren = "\U000f0654 " // nf-md-file_tree
	IconComments = "\U000f1801 " // nf-md-comment_text_multiple

	// Standard Unicode — TUI glyphs.
	GlyphChevron    = "\u276f" // ❯
	GlyphArrow      = "\u2192" // →
	GlyphReturn     = "\u21b3" // ↳
	GlyphEmDash     = "\u2014" // —
	GlyphDot        = "\u2022" // •
	GlyphHorizLine  = "\u2500" // ─
	GlyphVertLine   = "\u2502" // │
	GlyphCorner     = "\u2514" // └
	GlyphTee        = "\u251c" // ├
	GlyphTriangle   = "\u25b8" // ▸
	GlyphCircle     = "\u25cf" // ●
	GlyphArrowUp    = "\u2191" // ↑
	GlyphArrowDown  = "\u2193" // ↓
	GlyphInfinity   = "\u221e" // ∞
	GlyphBackspace  = "\u232b" // ⌫
	GlyphEscape     = "\u241b" // ␛
	GlyphCycleArrow = "\u27f3" // ⟳
)
