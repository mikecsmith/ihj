package document

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
)

// ANSIConfig controls rendering behavior.
type ANSIConfig struct {
	// WrapWidth is the maximum line width for paragraph text wrapping.
	// If zero, defaults to 80.
	WrapWidth int

	// Style is the glamour style used for terminal rendering.
	// If nil, falls back to glamour's environment config (GLAMOUR_STYLE
	// env var, or the built-in dark/light theme based on terminal background).
	Style *ansi.StyleConfig
}

// RenderANSI converts the AST into a styled string for terminal display.
// Internally converts the AST to markdown and renders it through glamour.
func RenderANSI(node *Node, cfg ANSIConfig) string {
	if node == nil {
		return ""
	}

	md := strings.TrimSpace(RenderMarkdown(node))
	if md == "" {
		return ""
	}

	width := cfg.WrapWidth
	if width <= 0 {
		width = 80
	}

	opts := []glamour.TermRendererOption{glamour.WithWordWrap(width)}
	if cfg.Style != nil {
		opts = append(opts, glamour.WithStyles(*cfg.Style))
	} else {
		opts = append(opts, glamour.WithEnvironmentConfig())
	}

	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		// Fallback: return unformatted markdown.
		return md + "\n"
	}

	out, err := r.Render(md)
	if err != nil {
		return md + "\n"
	}

	return out
}
