package theme

import ()

// TokyoNightTheme implements the Theme interface with Tokyo Night colors.
// It provides both dark and light variants.
type TokyoNightTheme struct {
	BaseTheme
}

// NewTokyoNightTheme creates a new instance of the Tokyo Night theme.
func NewTokyoNightTheme() *TokyoNightTheme {
	// Tokyo Night color palette
	// Dark mode colors
	darkBackground := "#222436"
	darkCurrentLine := "#1e2030"
	darkSelection := "#2f334d"
	darkForeground := "#c8d3f5"
	darkComment := "#636da6"
	darkRed := "#ff757f"
	darkOrange := "#ff966c"
	darkYellow := "#ffc777"
	darkGreen := "#c3e88d"
	darkCyan := "#86e1fc"
	darkBlue := "#82aaff"
	darkPurple := "#c099ff"
	darkBorder := "#3b4261"

	// Light mode colors (Tokyo Night Day)
	lightBackground := "#e1e2e7"
	lightCurrentLine := "#d5d6db"
	lightSelection := "#c8c9ce"
	lightForeground := "#3760bf"
	lightComment := "#848cb5"
	lightRed := "#f52a65"
	lightOrange := "#b15c00"
	lightYellow := "#8c6c3e"
	lightGreen := "#587539"
	lightCyan := "#007197"
	lightBlue := "#2e7de9"
	lightPurple := "#9854f1"
	lightBorder := "#a8aecb"

	theme := &TokyoNightTheme{}

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
		Dark:  "#191B29", // Darker background from palette
		Light: "#f0f0f5", // Slightly lighter than background
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
		Dark:  "#4fd6be", // teal from palette
		Light: "#1e725c",
	}
	theme.DiffRemovedColor = AdaptiveColor{
		Dark:  "#c53b53", // red1 from palette
		Light: "#c53b53",
	}
	theme.DiffContextColor = AdaptiveColor{
		Dark:  "#828bb8", // fg_dark from palette
		Light: "#7086b5",
	}
	theme.DiffHunkHeaderColor = AdaptiveColor{
		Dark:  "#828bb8", // fg_dark from palette
		Light: "#7086b5",
	}
	theme.DiffHighlightAddedColor = AdaptiveColor{
		Dark:  "#b8db87", // git.add from palette
		Light: "#4db380",
	}
	theme.DiffHighlightRemovedColor = AdaptiveColor{
		Dark:  "#e26a75", // git.delete from palette
		Light: "#f52a65",
	}
	theme.DiffAddedBgColor = AdaptiveColor{
		Dark:  "#20303b",
		Light: "#d5e5d5",
	}
	theme.DiffRemovedBgColor = AdaptiveColor{
		Dark:  "#37222c",
		Light: "#f7d8db",
	}
	theme.DiffContextBgColor = AdaptiveColor{
		Dark:  darkBackground,
		Light: lightBackground,
	}
	theme.DiffLineNumberColor = AdaptiveColor{
		Dark:  "#545c7e", // dark3 from palette
		Light: "#848cb5",
	}
	theme.DiffAddedLineNumberBgColor = AdaptiveColor{
		Dark:  "#1b2b34",
		Light: "#c5d5c5",
	}
	theme.DiffRemovedLineNumberBgColor = AdaptiveColor{
		Dark:  "#2d1f26",
		Light: "#e7c8cb",
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
	// Register the Tokyo Night theme with the theme manager
	RegisterTheme("tokyonight", NewTokyoNightTheme())
}
