package core

// Icons used across the TUI. Standard Unicode glyphs are centralised here
// so individual source files stay ASCII-clean. Each icon includes a trailing space.
const (
	IconUser        = "\u25c9 " // ◉ fisheye — assignee
	IconUserCard    = "\u25ce " // ◎ bullseye — reporter
	IconCalendar    = "\u25a3 " // ▣ square grid — dates
	IconRefresh     = "\u21bb " // ↻ clockwise arrow — refresh
	IconTag         = "\u266f " // ♯ sharp sign — labels/tags
	IconCube        = "\u25a1 " // □ white square — components
	IconTeam        = "\u2261 " // ≡ triple bar — team/group
	IconType        = "\u22a1 " // ⊡ squared dot — issue type
	IconParent      = "\u25b3 " // △ up triangle — parent link
	IconChildren    = "\u229e " // ⊞ squared plus — child issues
	IconComments    = "\u00a7 " // § section sign — comments
	IconFields      = "\u2263 " // ≣ four lines — field list
	IconField       = "\u25aa " // ▪ small black square — generic field fallback
	IconStoryPoints = "\u2295 " // ⊕ circled plus — story points / estimation
	IconSprint      = "\u23f1 " // ⏱ stopwatch — sprint / iteration

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
