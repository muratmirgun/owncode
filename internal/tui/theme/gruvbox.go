package theme

import ()

// Gruvbox color palette constants
const (
	// Dark theme colors
	gruvboxDarkBg0          = "#282828"
	gruvboxDarkBg0Soft      = "#32302f"
	gruvboxDarkBg1          = "#3c3836"
	gruvboxDarkBg2          = "#504945"
	gruvboxDarkBg3          = "#665c54"
	gruvboxDarkBg4          = "#7c6f64"
	gruvboxDarkFg0          = "#fbf1c7"
	gruvboxDarkFg1          = "#ebdbb2"
	gruvboxDarkFg2          = "#d5c4a1"
	gruvboxDarkFg3          = "#bdae93"
	gruvboxDarkFg4          = "#a89984"
	gruvboxDarkGray         = "#928374"
	gruvboxDarkRed          = "#cc241d"
	gruvboxDarkRedBright    = "#fb4934"
	gruvboxDarkGreen        = "#98971a"
	gruvboxDarkGreenBright  = "#b8bb26"
	gruvboxDarkYellow       = "#d79921"
	gruvboxDarkYellowBright = "#fabd2f"
	gruvboxDarkBlue         = "#458588"
	gruvboxDarkBlueBright   = "#83a598"
	gruvboxDarkPurple       = "#b16286"
	gruvboxDarkPurpleBright = "#d3869b"
	gruvboxDarkAqua         = "#689d6a"
	gruvboxDarkAquaBright   = "#8ec07c"
	gruvboxDarkOrange       = "#d65d0e"
	gruvboxDarkOrangeBright = "#fe8019"

	// Light theme colors
	gruvboxLightBg0          = "#fbf1c7"
	gruvboxLightBg0Soft      = "#f2e5bc"
	gruvboxLightBg1          = "#ebdbb2"
	gruvboxLightBg2          = "#d5c4a1"
	gruvboxLightBg3          = "#bdae93"
	gruvboxLightBg4          = "#a89984"
	gruvboxLightFg0          = "#282828"
	gruvboxLightFg1          = "#3c3836"
	gruvboxLightFg2          = "#504945"
	gruvboxLightFg3          = "#665c54"
	gruvboxLightFg4          = "#7c6f64"
	gruvboxLightGray         = "#928374"
	gruvboxLightRed          = "#9d0006"
	gruvboxLightRedBright    = "#cc241d"
	gruvboxLightGreen        = "#79740e"
	gruvboxLightGreenBright  = "#98971a"
	gruvboxLightYellow       = "#b57614"
	gruvboxLightYellowBright = "#d79921"
	gruvboxLightBlue         = "#076678"
	gruvboxLightBlueBright   = "#458588"
	gruvboxLightPurple       = "#8f3f71"
	gruvboxLightPurpleBright = "#b16286"
	gruvboxLightAqua         = "#427b58"
	gruvboxLightAquaBright   = "#689d6a"
	gruvboxLightOrange       = "#af3a03"
	gruvboxLightOrangeBright = "#d65d0e"
)

// GruvboxTheme implements the Theme interface with Gruvbox colors.
// It provides both dark and light variants.
type GruvboxTheme struct {
	BaseTheme
}

// NewGruvboxTheme creates a new instance of the Gruvbox theme.
func NewGruvboxTheme() *GruvboxTheme {
	theme := &GruvboxTheme{}

	// Base colors
	theme.PrimaryColor = AdaptiveColor{
		Dark:  gruvboxDarkBlueBright,
		Light: gruvboxLightBlueBright,
	}
	theme.SecondaryColor = AdaptiveColor{
		Dark:  gruvboxDarkPurpleBright,
		Light: gruvboxLightPurpleBright,
	}
	theme.AccentColor = AdaptiveColor{
		Dark:  gruvboxDarkOrangeBright,
		Light: gruvboxLightOrangeBright,
	}

	// Status colors
	theme.ErrorColor = AdaptiveColor{
		Dark:  gruvboxDarkRedBright,
		Light: gruvboxLightRedBright,
	}
	theme.WarningColor = AdaptiveColor{
		Dark:  gruvboxDarkYellowBright,
		Light: gruvboxLightYellowBright,
	}
	theme.SuccessColor = AdaptiveColor{
		Dark:  gruvboxDarkGreenBright,
		Light: gruvboxLightGreenBright,
	}
	theme.InfoColor = AdaptiveColor{
		Dark:  gruvboxDarkBlueBright,
		Light: gruvboxLightBlueBright,
	}

	// Text colors
	theme.TextColor = AdaptiveColor{
		Dark:  gruvboxDarkFg1,
		Light: gruvboxLightFg1,
	}
	theme.TextMutedColor = AdaptiveColor{
		Dark:  gruvboxDarkFg4,
		Light: gruvboxLightFg4,
	}
	theme.TextEmphasizedColor = AdaptiveColor{
		Dark:  gruvboxDarkYellowBright,
		Light: gruvboxLightYellowBright,
	}

	// Background colors
	theme.BackgroundColor = AdaptiveColor{
		Dark:  gruvboxDarkBg0,
		Light: gruvboxLightBg0,
	}
	theme.BackgroundSecondaryColor = AdaptiveColor{
		Dark:  gruvboxDarkBg1,
		Light: gruvboxLightBg1,
	}
	theme.BackgroundDarkerColor = AdaptiveColor{
		Dark:  gruvboxDarkBg0Soft,
		Light: gruvboxLightBg0Soft,
	}

	// Border colors
	theme.BorderNormalColor = AdaptiveColor{
		Dark:  gruvboxDarkBg2,
		Light: gruvboxLightBg2,
	}
	theme.BorderFocusedColor = AdaptiveColor{
		Dark:  gruvboxDarkBlueBright,
		Light: gruvboxLightBlueBright,
	}
	theme.BorderDimColor = AdaptiveColor{
		Dark:  gruvboxDarkBg1,
		Light: gruvboxLightBg1,
	}

	// Diff view colors
	theme.DiffAddedColor = AdaptiveColor{
		Dark:  gruvboxDarkGreenBright,
		Light: gruvboxLightGreenBright,
	}
	theme.DiffRemovedColor = AdaptiveColor{
		Dark:  gruvboxDarkRedBright,
		Light: gruvboxLightRedBright,
	}
	theme.DiffContextColor = AdaptiveColor{
		Dark:  gruvboxDarkFg4,
		Light: gruvboxLightFg4,
	}
	theme.DiffHunkHeaderColor = AdaptiveColor{
		Dark:  gruvboxDarkFg3,
		Light: gruvboxLightFg3,
	}
	theme.DiffHighlightAddedColor = AdaptiveColor{
		Dark:  gruvboxDarkGreenBright,
		Light: gruvboxLightGreenBright,
	}
	theme.DiffHighlightRemovedColor = AdaptiveColor{
		Dark:  gruvboxDarkRedBright,
		Light: gruvboxLightRedBright,
	}
	theme.DiffAddedBgColor = AdaptiveColor{
		Dark:  "#3C4C3C", // Darker green background
		Light: "#E8F5E9", // Light green background
	}
	theme.DiffRemovedBgColor = AdaptiveColor{
		Dark:  "#4C3C3C", // Darker red background
		Light: "#FFEBEE", // Light red background
	}
	theme.DiffContextBgColor = AdaptiveColor{
		Dark:  gruvboxDarkBg0,
		Light: gruvboxLightBg0,
	}
	theme.DiffLineNumberColor = AdaptiveColor{
		Dark:  gruvboxDarkFg4,
		Light: gruvboxLightFg4,
	}
	theme.DiffAddedLineNumberBgColor = AdaptiveColor{
		Dark:  "#32432F", // Slightly darker green
		Light: "#C8E6C9", // Light green
	}
	theme.DiffRemovedLineNumberBgColor = AdaptiveColor{
		Dark:  "#43322F", // Slightly darker red
		Light: "#FFCDD2", // Light red
	}

	// Markdown colors
	theme.MarkdownTextColor = AdaptiveColor{
		Dark:  gruvboxDarkFg1,
		Light: gruvboxLightFg1,
	}
	theme.MarkdownHeadingColor = AdaptiveColor{
		Dark:  gruvboxDarkYellowBright,
		Light: gruvboxLightYellowBright,
	}
	theme.MarkdownLinkColor = AdaptiveColor{
		Dark:  gruvboxDarkBlueBright,
		Light: gruvboxLightBlueBright,
	}
	theme.MarkdownLinkTextColor = AdaptiveColor{
		Dark:  gruvboxDarkAquaBright,
		Light: gruvboxLightAquaBright,
	}
	theme.MarkdownCodeColor = AdaptiveColor{
		Dark:  gruvboxDarkGreenBright,
		Light: gruvboxLightGreenBright,
	}
	theme.MarkdownBlockQuoteColor = AdaptiveColor{
		Dark:  gruvboxDarkAquaBright,
		Light: gruvboxLightAquaBright,
	}
	theme.MarkdownEmphColor = AdaptiveColor{
		Dark:  gruvboxDarkYellowBright,
		Light: gruvboxLightYellowBright,
	}
	theme.MarkdownStrongColor = AdaptiveColor{
		Dark:  gruvboxDarkOrangeBright,
		Light: gruvboxLightOrangeBright,
	}
	theme.MarkdownHorizontalRuleColor = AdaptiveColor{
		Dark:  gruvboxDarkBg3,
		Light: gruvboxLightBg3,
	}
	theme.MarkdownListItemColor = AdaptiveColor{
		Dark:  gruvboxDarkBlueBright,
		Light: gruvboxLightBlueBright,
	}
	theme.MarkdownListEnumerationColor = AdaptiveColor{
		Dark:  gruvboxDarkBlueBright,
		Light: gruvboxLightBlueBright,
	}
	theme.MarkdownImageColor = AdaptiveColor{
		Dark:  gruvboxDarkPurpleBright,
		Light: gruvboxLightPurpleBright,
	}
	theme.MarkdownImageTextColor = AdaptiveColor{
		Dark:  gruvboxDarkAquaBright,
		Light: gruvboxLightAquaBright,
	}
	theme.MarkdownCodeBlockColor = AdaptiveColor{
		Dark:  gruvboxDarkFg1,
		Light: gruvboxLightFg1,
	}

	// Syntax highlighting colors
	theme.SyntaxCommentColor = AdaptiveColor{
		Dark:  gruvboxDarkGray,
		Light: gruvboxLightGray,
	}
	theme.SyntaxKeywordColor = AdaptiveColor{
		Dark:  gruvboxDarkRedBright,
		Light: gruvboxLightRedBright,
	}
	theme.SyntaxFunctionColor = AdaptiveColor{
		Dark:  gruvboxDarkGreenBright,
		Light: gruvboxLightGreenBright,
	}
	theme.SyntaxVariableColor = AdaptiveColor{
		Dark:  gruvboxDarkBlueBright,
		Light: gruvboxLightBlueBright,
	}
	theme.SyntaxStringColor = AdaptiveColor{
		Dark:  gruvboxDarkYellowBright,
		Light: gruvboxLightYellowBright,
	}
	theme.SyntaxNumberColor = AdaptiveColor{
		Dark:  gruvboxDarkPurpleBright,
		Light: gruvboxLightPurpleBright,
	}
	theme.SyntaxTypeColor = AdaptiveColor{
		Dark:  gruvboxDarkYellow,
		Light: gruvboxLightYellow,
	}
	theme.SyntaxOperatorColor = AdaptiveColor{
		Dark:  gruvboxDarkAquaBright,
		Light: gruvboxLightAquaBright,
	}
	theme.SyntaxPunctuationColor = AdaptiveColor{
		Dark:  gruvboxDarkFg1,
		Light: gruvboxLightFg1,
	}

	return theme
}

func init() {
	// Register the Gruvbox theme with the theme manager
	RegisterTheme("gruvbox", NewGruvboxTheme())
}
