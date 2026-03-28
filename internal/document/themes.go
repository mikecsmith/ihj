package document

import (
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
)

// ContentTheme resolves a theme name to a glamour StyleConfig.
//
// Built-in glamour themes: "dark", "light", "dracula", "tokyo-night",
// "pink", "ascii", "notty".
//
// "default" returns ihj's custom theme, tuned for embedded content
// rendering (zero document margin, muted palette).
//
// Returns the default theme for empty or unrecognised names.
func ContentTheme(name string) *ansi.StyleConfig {
	if name == "" || name == "default" {
		return &defaultStyleConfig
	}
	if s, ok := styles.DefaultStyles[name]; ok {
		return s
	}
	return &defaultStyleConfig
}

// helpers for building style configs (glamour uses pointer fields).
func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }

// defaultStyleConfig is ihj's custom glamour theme. It uses standard ANSI
// 16-color codes so it adapts to the terminal's palette, matching the rest
// of the TUI. Document margin is 0 since content is rendered inside an
// existing layout (detail pane / comment block).
var defaultStyleConfig = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockPrefix: "\n",
			BlockSuffix: "\n",
		},
		Margin: uintPtr(0),
	},
	BlockQuote: ansi.StyleBlock{
		Indent:      uintPtr(1),
		IndentToken: stringPtr("│ "),
	},
	List: ansi.StyleList{
		LevelIndent: 2,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       stringPtr("6"), // Cyan
			Bold:        boolPtr(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "",
			Bold:   boolPtr(true),
			Upper:  boolPtr(true),
		},
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "## ",
		},
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "### ",
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "#### ",
		},
	},
	H5: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "##### ",
		},
	},
	H6: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "###### ",
			Bold:   boolPtr(false),
			Faint:  boolPtr(true),
		},
	},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: boolPtr(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: boolPtr(true),
	},
	Strong: ansi.StylePrimitive{
		Bold: boolPtr(true),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  stringPtr("8"), // Bright black / gray
		Format: "\n────────\n",
	},
	Item: ansi.StylePrimitive{
		BlockPrefix: "• ",
	},
	Enumeration: ansi.StylePrimitive{
		BlockPrefix: ". ",
	},
	Task: ansi.StyleTask{
		Ticked:   "[✓] ",
		Unticked: "[ ] ",
	},
	Link: ansi.StylePrimitive{
		Color:     stringPtr("4"), // Blue
		Underline: boolPtr(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: stringPtr("4"), // Blue
		Bold:  boolPtr(true),
	},
	Image: ansi.StylePrimitive{
		Color:     stringPtr("5"), // Magenta
		Underline: boolPtr(true),
	},
	ImageText: ansi.StylePrimitive{
		Color:  stringPtr("8"),
		Format: "Image: {{.text}} →",
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color:           stringPtr("6"), // Cyan
			BackgroundColor: stringPtr("8"), // Gray background
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("7"), // White
			},
			Margin: uintPtr(0),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: stringPtr("7"),
			},
			Comment: ansi.StylePrimitive{
				Color: stringPtr("8"),
			},
			Keyword: ansi.StylePrimitive{
				Color: stringPtr("4"), // Blue
			},
			KeywordType: ansi.StylePrimitive{
				Color: stringPtr("6"), // Cyan
			},
			Operator: ansi.StylePrimitive{
				Color: stringPtr("1"), // Red
			},
			Punctuation: ansi.StylePrimitive{
				Color: stringPtr("7"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: stringPtr("2"), // Green
			},
			NameClass: ansi.StylePrimitive{
				Color: stringPtr("6"), // Cyan
				Bold:  boolPtr(true),
			},
			NameTag: ansi.StylePrimitive{
				Color: stringPtr("5"), // Magenta
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: stringPtr("6"), // Cyan
			},
			LiteralString: ansi.StylePrimitive{
				Color: stringPtr("3"), // Yellow
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: stringPtr("6"), // Cyan
			},
			GenericDeleted: ansi.StylePrimitive{
				Color: stringPtr("1"), // Red
			},
			GenericInserted: ansi.StylePrimitive{
				Color: stringPtr("2"), // Green
			},
			GenericEmph: ansi.StylePrimitive{
				Italic: boolPtr(true),
			},
			GenericStrong: ansi.StylePrimitive{
				Bold: boolPtr(true),
			},
		},
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n> ",
	},
}
