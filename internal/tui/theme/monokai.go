package theme

import ()

// MonokaiProTheme implements the Theme interface with Monokai Pro colors.
// It provides both dark and light variants.
type MonokaiProTheme struct {
	BaseTheme
}

// NewMonokaiProTheme creates a new instance of the Monokai Pro theme.
func NewMonokaiProTheme() *MonokaiProTheme {
	// Monokai Pro color palette (dark mode)
	darkBackground := "#2d2a2e"
	darkCurrentLine := "#403e41"
	darkSelection := "#5b595c"
	darkForeground := "#fcfcfa"
	darkComment := "#727072"
	darkRed := "#ff6188"
	darkOrange := "#fc9867"
	darkYellow := "#ffd866"
	darkGreen := "#a9dc76"
	darkCyan := "#78dce8"
	darkBlue := "#ab9df2"
	darkPurple := "#ab9df2"
	darkBorder := "#403e41"

	// Light mode colors (adapted from dark)
	lightBackground := "#fafafa"
	lightCurrentLine := "#f0f0f0"
	lightSelection := "#e5e5e6"
	lightForeground := "#2d2a2e"
	lightComment := "#939293"
	lightRed := "#f92672"
	lightOrange := "#fd971f"
	lightYellow := "#e6db74"
	lightGreen := "#9bca65"
	lightCyan := "#66d9ef"
	lightBlue := "#7e75db"
	lightPurple := "#ae81ff"
	lightBorder := "#d3d3d3"

	theme := &MonokaiProTheme{}

	// Base colors
	theme.PrimaryColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.SecondaryColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.AccentColor = AdaptiveColor{
		Dark:  darkOrange,
		Light: lightOrange,
	}

	// Status colors
	theme.ErrorColor = AdaptiveColor{
		Dark:  darkRed,
		Light: lightRed,
	}
	theme.WarningColor = AdaptiveColor{
		Dark:  darkOrange,
		Light: lightOrange,
	}
	theme.SuccessColor = AdaptiveColor{
		Dark:  darkGreen,
		Light: lightGreen,
	}
	theme.InfoColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}

	// Text colors
	theme.TextColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}
	theme.TextMutedColor = AdaptiveColor{
		Dark:  darkComment,
		Light: lightComment,
	}
	theme.TextEmphasizedColor = AdaptiveColor{
		Dark:  darkYellow,
		Light: lightYellow,
	}

	// Background colors
	theme.BackgroundColor = AdaptiveColor{
		Dark:  darkBackground,
		Light: lightBackground,
	}
	theme.BackgroundSecondaryColor = AdaptiveColor{
		Dark:  darkCurrentLine,
		Light: lightCurrentLine,
	}
	theme.BackgroundDarkerColor = AdaptiveColor{
		Dark:  "#221f22", // Slightly darker than background
		Light: "#ffffff", // Slightly lighter than background
	}

	// Border colors
	theme.BorderNormalColor = AdaptiveColor{
		Dark:  darkBorder,
		Light: lightBorder,
	}
	theme.BorderFocusedColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.BorderDimColor = AdaptiveColor{
		Dark:  darkSelection,
		Light: lightSelection,
	}

	// Diff view colors
	theme.DiffAddedColor = AdaptiveColor{
		Dark:  "#a9dc76",
		Light: "#9bca65",
	}
	theme.DiffRemovedColor = AdaptiveColor{
		Dark:  "#ff6188",
		Light: "#f92672",
	}
	theme.DiffContextColor = AdaptiveColor{
		Dark:  "#a0a0a0",
		Light: "#757575",
	}
	theme.DiffHunkHeaderColor = AdaptiveColor{
		Dark:  "#a0a0a0",
		Light: "#757575",
	}
	theme.DiffHighlightAddedColor = AdaptiveColor{
		Dark:  "#c2e7a9",
		Light: "#c5e0b4",
	}
	theme.DiffHighlightRemovedColor = AdaptiveColor{
		Dark:  "#ff8ca6",
		Light: "#ffb3c8",
	}
	theme.DiffAddedBgColor = AdaptiveColor{
		Dark:  "#3a4a35",
		Light: "#e8f5e9",
	}
	theme.DiffRemovedBgColor = AdaptiveColor{
		Dark:  "#4a3439",
		Light: "#ffebee",
	}
	theme.DiffContextBgColor = AdaptiveColor{
		Dark:  darkBackground,
		Light: lightBackground,
	}
	theme.DiffLineNumberColor = AdaptiveColor{
		Dark:  "#888888",
		Light: "#9e9e9e",
	}
	theme.DiffAddedLineNumberBgColor = AdaptiveColor{
		Dark:  "#2d3a28",
		Light: "#c8e6c9",
	}
	theme.DiffRemovedLineNumberBgColor = AdaptiveColor{
		Dark:  "#3d2a2e",
		Light: "#ffcdd2",
	}

	// Markdown colors
	theme.MarkdownTextColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}
	theme.MarkdownHeadingColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.MarkdownLinkColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.MarkdownLinkTextColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.MarkdownCodeColor = AdaptiveColor{
		Dark:  darkGreen,
		Light: lightGreen,
	}
	theme.MarkdownBlockQuoteColor = AdaptiveColor{
		Dark:  darkYellow,
		Light: lightYellow,
	}
	theme.MarkdownEmphColor = AdaptiveColor{
		Dark:  darkYellow,
		Light: lightYellow,
	}
	theme.MarkdownStrongColor = AdaptiveColor{
		Dark:  darkOrange,
		Light: lightOrange,
	}
	theme.MarkdownHorizontalRuleColor = AdaptiveColor{
		Dark:  darkComment,
		Light: lightComment,
	}
	theme.MarkdownListItemColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.MarkdownListEnumerationColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.MarkdownImageColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.MarkdownImageTextColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.MarkdownCodeBlockColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}

	// Syntax highlighting colors
	theme.SyntaxCommentColor = AdaptiveColor{
		Dark:  darkComment,
		Light: lightComment,
	}
	theme.SyntaxKeywordColor = AdaptiveColor{
		Dark:  darkRed,
		Light: lightRed,
	}
	theme.SyntaxFunctionColor = AdaptiveColor{
		Dark:  darkGreen,
		Light: lightGreen,
	}
	theme.SyntaxVariableColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}
	theme.SyntaxStringColor = AdaptiveColor{
		Dark:  darkYellow,
		Light: lightYellow,
	}
	theme.SyntaxNumberColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.SyntaxTypeColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.SyntaxOperatorColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.SyntaxPunctuationColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}

	return theme
}

func init() {
	// Register the Monokai Pro theme with the theme manager
	RegisterTheme("monokai", NewMonokaiProTheme())
}
