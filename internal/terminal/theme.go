package terminal

import (
	"image/color"
	"strings"

	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Theme defines the complete visual language for the TUI.
// Every styled element in the application references this rather than
// defining colors inline. Change the palette here to re-skin everything.
type Theme struct {
	Accent  color.Color
	Muted   color.Color
	Surface color.Color
	Overlay color.Color
	Text    color.Color

	// Semantic colors.
	Success color.Color
	Warning color.Color
	Error   color.Color
	Info    color.Color

	// Issue type colors.
	TypeInitiative color.Color
	TypeEpic       color.Color
	TypeStory      color.Color
	TypeTask       color.Color
	TypeBug        color.Color
	TypeSubtask    color.Color

	// Status colors.
	StatusDone    color.Color
	StatusActive  color.Color
	StatusReview  color.Color
	StatusReady   color.Color
	StatusBlocked color.Color
	StatusDefault color.Color
}

// DefaultTheme returns the standard ihj color scheme.
// Uses standard 16-color ANSI codes so colors adapt to the terminal theme,
// matching the original Python TUI's use of \033[3Xm sequences.
func DefaultTheme() *Theme {
	return &Theme{
		Accent:  lipgloss.Color("4"), // Blue (\033[34m)
		Muted:   lipgloss.Color("8"), // Bright black / gray (\033[90m)
		Surface: lipgloss.Color("0"), // Black background (\033[40m)
		Overlay: lipgloss.Color("8"), // Gray background
		Text:    lipgloss.Color("7"), // White / default (\033[37m)

		Success: lipgloss.Color("2"), // Green (\033[32m)
		Warning: lipgloss.Color("3"), // Yellow (\033[33m)
		Error:   lipgloss.Color("1"), // Red (\033[31m)
		Info:    lipgloss.Color("6"), // Cyan (\033[36m)

		TypeInitiative: lipgloss.Color("6"), // Cyan (\033[36m)
		TypeEpic:       lipgloss.Color("5"), // Magenta (\033[35m)
		TypeStory:      lipgloss.Color("4"), // Blue (\033[34m)
		TypeTask:       lipgloss.Color("7"), // White / default (\033[37m)
		TypeBug:        lipgloss.Color("1"), // Red (\033[31m)
		TypeSubtask:    lipgloss.Color("7"), // White (\033[37m)

		StatusDone:    lipgloss.Color("2"), // Green
		StatusActive:  lipgloss.Color("4"), // Blue
		StatusReview:  lipgloss.Color("5"), // Magenta
		StatusReady:   lipgloss.Color("6"), // Cyan
		StatusBlocked: lipgloss.Color("1"), // Red
		StatusDefault: lipgloss.Color("7"), // White
	}
}

// Styles holds all pre-computed Lip Gloss styles derived from a Theme.
type Styles struct {
	DynamicTypeColors   map[string]color.Color
	DynamicStatusColors map[string]color.Color
	// Layout.
	AppBorder      lipgloss.Style
	StatusBar      lipgloss.Style
	StatusBarKey   lipgloss.Style
	StatusBarValue lipgloss.Style
	HelpBar        lipgloss.Style

	// List items.
	IssueKey     lipgloss.Style
	IssueKeyDim  lipgloss.Style
	Summary      lipgloss.Style
	SummaryChild lipgloss.Style
	ChildCount   lipgloss.Style
	TreeGlyph    lipgloss.Style
	ColumnHeader lipgloss.Style
	Cursor       lipgloss.Style

	// Detail pane.
	DetailHeader  lipgloss.Style
	DetailDivider lipgloss.Style
	CommentAuthor lipgloss.Style
	CommentDate   lipgloss.Style
	ActionKey     lipgloss.Style
	ActionDesc    lipgloss.Style

	// Detail metadata values.
	DetailValue lipgloss.Style

	// Label width for metadata section (shared by MetadataLabelStyle).
	labelW int
	theme  *Theme

	// Section headers — different colors per section.
	ChildSection   lipgloss.Style // Blue bold (C['blue']+C['bold'])
	CommentSection lipgloss.Style // Yellow bold (C['yellow']+C['bold'])

	// Inline input.
	InputLabel lipgloss.Style

	// Notifications.
	NotifySuccess lipgloss.Style
	NotifyError   lipgloss.Style
	NotifyInfo    lipgloss.Style

	// Priority icons.
	PrioCritical lipgloss.Style
	PrioHigh     lipgloss.Style
	PrioMedium   lipgloss.Style
	PrioLow      lipgloss.Style
	PrioTrivial  lipgloss.Style

	// Content rendering — glamour style for descriptions and comments.
	ContentStyle *ansi.StyleConfig
}

// NewStyles builds the complete style set from a theme.
func NewStyles(t *Theme, ws *core.Workspace, contentTheme string) *Styles {
	dim := lipgloss.NewStyle().Faint(true)
	accent := lipgloss.NewStyle().Foreground(t.Accent)

	// Build dynamic color maps from workspace config.
	dynamicTypeColors := make(map[string]color.Color)
	dynamicStatusColors := make(map[string]color.Color)
	if ws != nil {
		for name, entry := range ws.TypeOrderMap {
			dynamicTypeColors[name] = parseColorString(entry.Color, t)
		}
		for name, entry := range ws.StatusOrderMap {
			dynamicStatusColors[name] = parseColorString(entry.Color, t)
		}
	}

	labelW := 15

	return &Styles{
		DynamicTypeColors:   dynamicTypeColors,
		DynamicStatusColors: dynamicStatusColors,
		// Layout.
		AppBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Muted).
			Padding(1, 2),
		StatusBar: lipgloss.NewStyle().
			Background(t.Surface).
			Padding(0, 1),
		StatusBarKey: lipgloss.NewStyle().
			Foreground(t.Accent).Bold(true),
		StatusBarValue: dim,
		HelpBar: lipgloss.NewStyle().
			Faint(true),

		// List.
		IssueKey:     lipgloss.NewStyle().Bold(true),
		IssueKeyDim:  lipgloss.NewStyle().Faint(true),
		Summary:      lipgloss.NewStyle(),
		SummaryChild: lipgloss.NewStyle(),
		ChildCount:   dim,
		TreeGlyph:    dim,
		ColumnHeader: lipgloss.NewStyle().Bold(true),
		Cursor: lipgloss.NewStyle().
			Background(t.Overlay).
			Bold(true),

		// Detail.
		DetailHeader:  lipgloss.NewStyle().Bold(true),
		DetailDivider: dim,
		CommentAuthor: lipgloss.NewStyle().Bold(true),
		CommentDate:   dim,
		ActionKey: lipgloss.NewStyle().
			Foreground(t.Info).Bold(true),
		ActionDesc: dim,

		// Detail metadata.
		DetailValue: lipgloss.NewStyle(),
		labelW:      labelW,
		theme:       t,

		// Section headers — different colors per section.
		ChildSection:   lipgloss.NewStyle().Bold(true).Foreground(t.Accent),  // Blue bold
		CommentSection: lipgloss.NewStyle().Bold(true).Foreground(t.Warning), // Yellow bold

		// Input.
		InputLabel: accent.Bold(true),

		// Notifications.
		NotifySuccess: lipgloss.NewStyle().Foreground(t.Success),
		NotifyError:   lipgloss.NewStyle().Foreground(t.Error).Bold(true),
		NotifyInfo:    lipgloss.NewStyle().Foreground(t.Info),

		// Priority.
		PrioCritical: lipgloss.NewStyle().Foreground(t.Error).Bold(true),
		PrioHigh:     lipgloss.NewStyle().Foreground(t.Error),
		PrioMedium:   lipgloss.NewStyle().Foreground(t.Warning),
		PrioLow:      lipgloss.NewStyle().Foreground(t.Accent),
		PrioTrivial:  lipgloss.NewStyle().Foreground(t.Info),

		// Content — resolved from config theme name.
		ContentStyle: document.ContentTheme(contentTheme),
	}
}

// categColors is the fixed color cycle for categorisation fields.
var categColors = func(t *Theme) []color.Color {
	return []color.Color{t.Accent, t.TypeEpic, t.Info}
}

// MetadataLabelStyle returns the style for a field's label in the detail pane,
// based on its Role, whether it's Primary, and its position within the role group.
func (s *Styles) MetadataLabelStyle(role core.FieldRole, primary bool, indexInRole int) lipgloss.Style {
	base := lipgloss.NewStyle().Width(s.labelW)
	switch role {
	case core.RoleOwnership:
		if primary {
			return base.Foreground(s.theme.Info) // Cyan
		}
		return base.Faint(true) // Dim
	case core.RoleCategorisation:
		colors := categColors(s.theme)
		c := colors[indexInRole%len(colors)]
		return base.Foreground(c)
	default:
		// Temporal, Urgency, Iteration, Default — all dim.
		return base.Faint(true)
	}
}

// ParentLabelStyle returns the style for the structural parent label.
func (s *Styles) ParentLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Faint(true).Width(s.labelW)
}

// TypeColor returns the color for a given issue type name.
func (s *Styles) TypeColor(typeName string) color.Color {
	lower := strings.ToLower(typeName)

	// Check if the board config provided a custom color for this exact type
	if customColor, exists := s.DynamicTypeColors[lower]; exists {
		return customColor
	}

	// Fallback to defaults
	t := DefaultTheme()
	switch lower {
	case "initiative":
		return t.TypeInitiative
	case "epic":
		return t.TypeEpic
	case "story":
		return t.TypeStory
	case "bug":
		return t.TypeBug
	case "sub-task", "subtask":
		return t.TypeSubtask
	default:
		return t.TypeTask
	}
}

// TypeIcon returns an icon for the identity bar.
// Uses simple Unicode — the original Python TUI didn't use type icons,
// just colored uppercase text. This adds a small visual marker.
func (s *Styles) TypeIcon(typeName string) string {
	switch strings.ToLower(typeName) {
	case "initiative":
		return "⟡"
	case "epic":
		return "⚡"
	case "story":
		return "◆"
	case "bug":
		return "●"
	case "sub-task", "subtask":
		return "◇"
	case "task":
		return "▪"
	default:
		return "○"
	}
}

// TypeBadge returns a styled badge for an issue type.
func (s *Styles) TypeBadge(typeName string) string {
	color := s.TypeColor(typeName)
	return lipgloss.NewStyle().Foreground(color).Render(typeName)
}

// StatusStyle returns icon and color for a status name.
// Color is resolved from workspace config first, falling back to heuristic defaults.
func (s *Styles) StatusStyle(status string) (string, color.Color) {
	lower := strings.ToLower(status)
	icon := statusIcon(lower)

	// Use workspace-configured color if available.
	if c, ok := s.DynamicStatusColors[lower]; ok {
		return icon, c
	}

	// Fallback to heuristic defaults.
	t := DefaultTheme()
	switch {
	case containsAny(lower, "done", "closed", "resolved", "complete"):
		return icon, t.StatusDone
	case containsAny(lower, "block", "stop", "hold", "cancel"):
		return icon, t.StatusBlocked
	case containsAny(lower, "review", "test", "qa", "verification"):
		return icon, t.StatusReview
	case containsAny(lower, "progress", "doing", "active", "dev"):
		return icon, t.StatusActive
	case containsAny(lower, "refined", "ready", "approved"):
		return icon, t.StatusReady
	default:
		return icon, t.StatusDefault
	}
}

// statusIcon returns an icon for a status based on heuristic name matching.
func statusIcon(lower string) string {
	switch {
	case containsAny(lower, "done", "closed", "resolved", "complete"):
		return "✔"
	case containsAny(lower, "block", "stop", "hold", "cancel"):
		return "✘"
	case containsAny(lower, "review", "test", "qa", "verification"):
		return "◉"
	case containsAny(lower, "progress", "doing", "active", "dev"):
		return "▶"
	case containsAny(lower, "refined", "ready", "approved"):
		return "★"
	default:
		return "○"
	}
}

// StatusBadge returns a styled badge for a status.
func (s *Styles) StatusBadge(status string) string {
	icon, color := s.StatusStyle(status)
	return lipgloss.NewStyle().Foreground(color).Render(icon + " " + status)
}

// PriorityIcon returns a styled priority indicator.
func (s *Styles) PriorityIcon(priority string) string {
	lower := strings.ToLower(priority)
	switch {
	case containsAny(lower, "crit", "highest", "blocker"):
		return s.PrioCritical.Render("▲")
	case containsAny(lower, "major", "high"):
		return s.PrioHigh.Render("▴")
	case containsAny(lower, "medium"):
		return s.PrioMedium.Render("◆")
	case containsAny(lower, "minor", "low"):
		return s.PrioLow.Render("▾")
	case containsAny(lower, "lowest", "trivial"):
		return s.PrioTrivial.Render("▼")
	default:
		return lipgloss.NewStyle().Foreground(DefaultTheme().Muted).Render("−")
	}
}

// PriorityIconWithBg renders the priority icon with an optional cursor
// background when the row is selected, so the highlight bar is continuous.
func (s *Styles) PriorityIconWithBg(priority string, selected bool) string {
	withBg := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Background(s.Cursor.GetBackground())
		}
		return st
	}
	lower := strings.ToLower(priority)
	switch {
	case containsAny(lower, "crit", "highest", "blocker"):
		return withBg(s.PrioCritical).Render("▲")
	case containsAny(lower, "major", "high"):
		return withBg(s.PrioHigh).Render("▴")
	case containsAny(lower, "medium"):
		return withBg(s.PrioMedium).Render("◆")
	case containsAny(lower, "minor", "low"):
		return withBg(s.PrioLow).Render("▾")
	case containsAny(lower, "lowest", "trivial"):
		return withBg(s.PrioTrivial).Render("▼")
	default:
		return withBg(lipgloss.NewStyle().Foreground(DefaultTheme().Muted)).Render("−")
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// parseColorString converts a string a Lipgloss color,
// falling back to the default theme if the string is empty or invalid.
func parseColorString(c string, t *Theme) color.Color {
	switch strings.ToLower(c) {
	case "black":
		return lipgloss.Color("0")
	case "red":
		return t.Error
	case "green":
		return t.Success
	case "yellow":
		return t.Warning
	case "blue":
		return t.Accent
	case "magenta":
		return t.TypeEpic
	case "cyan":
		return t.Info
	case "white":
		return t.Text
	case "dim":
		return t.Muted
	default:
		// Treat it as an ANSI number or hex string if it doesn't match a known name
		if c != "" {
			return lipgloss.Color(c)
		}
		return t.Text
	}
}
