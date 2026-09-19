package theme

import ()

// DraculaTheme implements the Theme interface with Dracula colors.
// It provides both dark and light variants, though Dracula is primarily a dark theme.
type DraculaTheme struct {
	BaseTheme
}

// NewDraculaTheme creates a new instance of the Dracula theme.
func NewDraculaTheme() *DraculaTheme {
	// Dracula color palette
	// Official colors from https://draculatheme.com/
	darkBackground := "#282a36"
	darkCurrentLine := "#44475a"
	darkSelection := "#44475a"
	darkForeground := "#f8f8f2"
	darkComment := "#6272a4"
	darkCyan := "#8be9fd"
	darkGreen := "#50fa7b"
	darkOrange := "#ffb86c"
	darkPink := "#ff79c6"
	darkPurple := "#bd93f9"
	darkRed := "#ff5555"
	darkYellow := "#f1fa8c"
	darkBorder := "#44475a"

	// Light mode approximation (Dracula is primarily a dark theme)
	lightBackground := "#f8f8f2"
	lightCurrentLine := "#e6e6e6"
	lightSelection := "#d8d8d8"
	lightForeground := "#282a36"
	lightComment := "#6272a4"
	lightCyan := "#0097a7"
	lightGreen := "#388e3c"
	lightOrange := "#f57c00"
	lightPink := "#d81b60"
	lightPurple := "#7e57c2"
	lightRed := "#e53935"
	lightYellow := "#fbc02d"
	lightBorder := "#d8d8d8"

	theme := &DraculaTheme{}

	// Base colors
	theme.PrimaryColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.SecondaryColor = AdaptiveColor{
		Dark:  darkPink,
		Light: lightPink,
	}
	theme.AccentColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
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
		Dark:  "#21222c", // Slightly darker than background
		Light: "#ffffff", // Slightly lighter than background
	}

	// Border colors
	theme.BorderNormalColor = AdaptiveColor{
		Dark:  darkBorder,
		Light: lightBorder,
	}
	theme.BorderFocusedColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
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
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.DiffHighlightAddedColor = AdaptiveColor{
		Dark:  "#50fa7b",
		Light: "#a5d6a7",
	}
	theme.DiffHighlightRemovedColor = AdaptiveColor{
		Dark:  "#ff5555",
		Light: "#ef9a9a",
	}
	theme.DiffAddedBgColor = AdaptiveColor{
		Dark:  "#2c3b2c",
		Light: "#e8f5e9",
	}
	theme.DiffRemovedBgColor = AdaptiveColor{
		Dark:  "#3b2c2c",
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
		Dark:  "#253025",
		Light: "#c8e6c9",
	}
	theme.DiffRemovedLineNumberBgColor = AdaptiveColor{
		Dark:  "#302525",
		Light: "#ffcdd2",
	}

	// Markdown colors
	theme.MarkdownTextColor = AdaptiveColor{
		Dark:  darkForeground,
		Light: lightForeground,
	}
	theme.MarkdownHeadingColor = AdaptiveColor{
		Dark:  darkPink,
		Light: lightPink,
	}
	theme.MarkdownLinkColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
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
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.MarkdownListEnumerationColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
	}
	theme.MarkdownImageColor = AdaptiveColor{
		Dark:  darkPurple,
		Light: lightPurple,
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
		Dark:  darkPink,
		Light: lightPink,
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
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.SyntaxTypeColor = AdaptiveColor{
		Dark:  darkCyan,
		Light: lightCyan,
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
	// Register the Dracula theme with the theme manager
	RegisterTheme("dracula", NewDraculaTheme())
}
