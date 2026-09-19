package theme

import ()

// OneDarkTheme implements the Theme interface with Atom's One Dark colors.
// It provides both dark and light variants.
type OneDarkTheme struct {
	BaseTheme
}

// NewOneDarkTheme creates a new instance of the One Dark theme.
func NewOneDarkTheme() *OneDarkTheme {
	// One Dark color palette
	// Dark mode colors from Atom One Dark
	darkBackground := "#282c34"
	darkCurrentLine := "#2c313c"
	darkSelection := "#3e4451"
	darkForeground := "#abb2bf"
	darkComment := "#5c6370"
	darkRed := "#e06c75"
	darkOrange := "#d19a66"
	darkYellow := "#e5c07b"
	darkGreen := "#98c379"
	darkCyan := "#56b6c2"
	darkBlue := "#61afef"
	darkPurple := "#c678dd"
	darkBorder := "#3b4048"

	// Light mode colors from Atom One Light
	lightBackground := "#fafafa"
	lightCurrentLine := "#f0f0f0"
	lightSelection := "#e5e5e6"
	lightForeground := "#383a42"
	lightComment := "#a0a1a7"
	lightRed := "#e45649"
	lightOrange := "#da8548"
	lightYellow := "#c18401"
	lightGreen := "#50a14f"
	lightCyan := "#0184bc"
	lightBlue := "#4078f2"
	lightPurple := "#a626a4"
	lightBorder := "#d3d3d3"

	theme := &OneDarkTheme{}

	// Base colors
	theme.PrimaryColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
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
		Dark:  "#21252b", // Slightly darker than background
		Light: "#ffffff", // Slightly lighter than background
	}

	// Border colors
	theme.BorderNormalColor = AdaptiveColor{
		Dark:  darkBorder,
		Light: lightBorder,
	}
	theme.BorderFocusedColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.BorderDimColor = AdaptiveColor{
		Dark:  darkSelection,
		Light: lightSelection,
	}

	// Diff view colors
	theme.DiffAddedColor = AdaptiveColor{
		Dark:  "#478247",
		Light: "#2E7D32",
	}
	theme.DiffRemovedColor = AdaptiveColor{
		Dark:  "#7C4444",
		Light: "#C62828",
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
		Dark:  "#DAFADA",
		Light: "#A5D6A7",
	}
	theme.DiffHighlightRemovedColor = AdaptiveColor{
		Dark:  "#FADADD",
		Light: "#EF9A9A",
	}
	theme.DiffAddedBgColor = AdaptiveColor{
		Dark:  "#303A30",
		Light: "#E8F5E9",
	}
	theme.DiffRemovedBgColor = AdaptiveColor{
		Dark:  "#3A3030",
		Light: "#FFEBEE",
	}
	theme.DiffContextBgColor = AdaptiveColor{
		Dark:  darkBackground,
		Light: lightBackground,
	}
	theme.DiffLineNumberColor = AdaptiveColor{
		Dark:  "#888888",
		Light: "#9E9E9E",
	}
	theme.DiffAddedLineNumberBgColor = AdaptiveColor{
		Dark:  "#293229",
		Light: "#C8E6C9",
	}
	theme.DiffRemovedLineNumberBgColor = AdaptiveColor{
		Dark:  "#332929",
		Light: "#FFCDD2",
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
		Dark:  darkPurple,
		Light: lightPurple,
	}
	theme.SyntaxFunctionColor = AdaptiveColor{
		Dark:  darkBlue,
		Light: lightBlue,
	}
	theme.SyntaxVariableColor = AdaptiveColor{
		Dark:  darkRed,
		Light: lightRed,
	}
	theme.SyntaxStringColor = AdaptiveColor{
		Dark:  darkGreen,
		Light: lightGreen,
	}
	theme.SyntaxNumberColor = AdaptiveColor{
		Dark:  darkOrange,
		Light: lightOrange,
	}
	theme.SyntaxTypeColor = AdaptiveColor{
		Dark:  darkYellow,
		Light: lightYellow,
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
	// Register the One Dark theme with the theme manager
	RegisterTheme("onedark", NewOneDarkTheme())
}
