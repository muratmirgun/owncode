package theme

import ()

// TronTheme implements the Theme interface with Tron-inspired colors.
// It provides both dark and light variants, though Tron is primarily a dark theme.
type TronTheme struct {
	BaseTheme
}

// NewTronTheme creates a new instance of the Tron theme.
func NewTronTheme() *TronTheme {
	// Tron color palette
	// Inspired by the Tron movie's neon aesthetic
	darkBackground := "#0c141f"
	darkCurrentLine := "#1a2633"
	darkSelection := "#1a2633"
	darkForeground := "#caf0ff"
	darkComment := "#4d6b87"
	darkCyan := "#00d9ff"
	darkBlue := "#007fff"
	darkOrange := "#ff9000"
	darkPink := "#ff00a0"
	darkPurple := "#b73fff"
	darkRed := "#ff3333"
	darkYellow := "#ffcc00"
	darkGreen := "#00ff8f"
	darkBorder := "#1a2633"

	// Light mode approximation
	lightBackground := "#f0f8ff"
	lightCurrentLine := "#e0f0ff"
	lightSelection := "#d0e8ff"
	lightForeground := "#0c141f"
	lightComment := "#4d6b87"
	lightCyan := "#0097b3"
	lightBlue := "#0066cc"
	lightOrange := "#cc7300"
	lightPink := "#cc0080"
	lightPurple := "#9932cc"
	lightRed := "#cc2929"
	lightYellow := "#cc9900"
	lightGreen := "#00cc72"
	lightBorder := "#d0e8ff"

	theme := &TronTheme{}

	// Base colors
	theme.PrimaryColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.SecondaryColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
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
		Dark:  darkCyan,
		Light: lightCyan,
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
		Dark:  "#070d14", // Slightly darker than background
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
		Dark:  darkGreen,
		Light: lightGreen,
	}
	theme.DiffRemovedColor = AdaptiveColor{
		Dark:  darkRed,
		Light: lightRed,
	}
	theme.DiffContextColor = AdaptiveColor{
		Dark:  darkComment,
		Light: lightComment,
	}
	theme.DiffHunkHeaderColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.DiffHighlightAddedColor = AdaptiveColor{
		Dark:  "#00ff8f",
		Light: "#a5d6a7",
	}
	theme.DiffHighlightRemovedColor = AdaptiveColor{
		Dark:  "#ff3333",
		Light: "#ef9a9a",
	}
	theme.DiffAddedBgColor = AdaptiveColor{
		Dark:  "#0a2a1a",
		Light: "#e8f5e9",
	}
	theme.DiffRemovedBgColor = AdaptiveColor{
		Dark:  "#2a0a0a",
		Light: "#ffebee",
	}
	theme.DiffContextBgColor = AdaptiveColor{
		Dark:  darkBackground,
		Light: lightBackground,
	}
	theme.DiffLineNumberColor = AdaptiveColor{
		Dark:  darkComment,
		Light: lightComment,
	}
	theme.DiffAddedLineNumberBgColor = AdaptiveColor{
		Dark:  "#082015",
		Light: "#c8e6c9",
	}
	theme.DiffRemovedLineNumberBgColor = AdaptiveColor{
		Dark:  "#200808",
		Light: "#ffcdd2",
	}

	// Markdown colors
	theme.MarkdownTextColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}
	theme.MarkdownHeadingColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.MarkdownLinkColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.MarkdownLinkTextColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
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
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.MarkdownListEnumerationColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.MarkdownImageColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.MarkdownImageTextColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
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
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.SyntaxFunctionColor = AdaptiveColor{
		Dark:  darkGreen,
		Light: lightGreen,
	}
	theme.SyntaxVariableColor = AdaptiveColor{
		Dark:  darkOrange,
		Light: lightOrange,
	}
	theme.SyntaxStringColor = AdaptiveColor{
		Dark:  darkYellow,
		Light: lightYellow,
	}
	theme.SyntaxNumberColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.SyntaxTypeColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.SyntaxOperatorColor = AdaptiveColor{
		Dark:  darkPink,
		Light: lightPink,
	}
	theme.SyntaxPunctuationColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}

	return theme
}

func init() {
	// Register the Tron theme with the theme manager
	RegisterTheme("tron", NewTronTheme())
}
