package theme

import (
	catppuccin "github.com/catppuccin/go"
)

// CatppuccinTheme implements the Theme interface with Catppuccin colors.
// It provides both dark (Mocha) and light (Latte) variants.
type CatppuccinTheme struct {
	BaseTheme
}

// NewCatppuccinTheme creates a new instance of the Catppuccin theme.
func NewCatppuccinTheme() *CatppuccinTheme {
	// Get the Catppuccin palettes
	mocha := catppuccin.Mocha
	latte := catppuccin.Latte

	theme := &CatppuccinTheme{}

	// Base colors
	theme.PrimaryColor = AdaptiveColor{
		Dark:  mocha.Blue().Hex,
		Light: latte.Blue().Hex,
	}
	theme.SecondaryColor = AdaptiveColor{
		Dark:  mocha.Mauve().Hex,
		Light: latte.Mauve().Hex,
	}
	theme.AccentColor = AdaptiveColor{
		Dark:  mocha.Peach().Hex,
		Light: latte.Peach().Hex,
	}

	// Status colors
	theme.ErrorColor = AdaptiveColor{
		Dark:  mocha.Red().Hex,
		Light: latte.Red().Hex,
	}
	theme.WarningColor = AdaptiveColor{
		Dark:  mocha.Peach().Hex,
		Light: latte.Peach().Hex,
	}
	theme.SuccessColor = AdaptiveColor{
		Dark:  mocha.Green().Hex,
		Light: latte.Green().Hex,
	}
	theme.InfoColor = AdaptiveColor{
		Dark:  mocha.Blue().Hex,
		Light: latte.Blue().Hex,
	}

	// Text colors
	theme.TextColor = AdaptiveColor{
		Dark:  mocha.Text().Hex,
		Light: latte.Text().Hex,
	}
	theme.TextMutedColor = AdaptiveColor{
		Dark:  mocha.Subtext0().Hex,
		Light: latte.Subtext0().Hex,
	}
	theme.TextEmphasizedColor = AdaptiveColor{
		Dark:  mocha.Lavender().Hex,
		Light: latte.Lavender().Hex,
	}

	// Background colors
	theme.BackgroundColor = AdaptiveColor{
		Dark:  "#212121", // From existing styles
		Light: "#EEEEEE", // Light equivalent
	}
	theme.BackgroundSecondaryColor = AdaptiveColor{
		Dark:  "#2c2c2c", // From existing styles
		Light: "#E0E0E0", // Light equivalent
	}
	theme.BackgroundDarkerColor = AdaptiveColor{
		Dark:  "#181818", // From existing styles
		Light: "#F5F5F5", // Light equivalent
	}

	// Border colors
	theme.BorderNormalColor = AdaptiveColor{
		Dark:  "#4b4c5c", // From existing styles
		Light: "#BDBDBD", // Light equivalent
	}
	theme.BorderFocusedColor = AdaptiveColor{
		Dark:  mocha.Blue().Hex,
		Light: latte.Blue().Hex,
	}
	theme.BorderDimColor = AdaptiveColor{
		Dark:  mocha.Surface0().Hex,
		Light: latte.Surface0().Hex,
	}

	// Diff view colors
	theme.DiffAddedColor = AdaptiveColor{
		Dark:  "#478247", // From existing diff.go
		Light: "#2E7D32", // Light equivalent
	}
	theme.DiffRemovedColor = AdaptiveColor{
		Dark:  "#7C4444", // From existing diff.go
		Light: "#C62828", // Light equivalent
	}
	theme.DiffContextColor = AdaptiveColor{
		Dark:  "#a0a0a0", // From existing diff.go
		Light: "#757575", // Light equivalent
	}
	theme.DiffHunkHeaderColor = AdaptiveColor{
		Dark:  "#a0a0a0", // From existing diff.go
		Light: "#757575", // Light equivalent
	}
	theme.DiffHighlightAddedColor = AdaptiveColor{
		Dark:  "#DAFADA", // From existing diff.go
		Light: "#A5D6A7", // Light equivalent
	}
	theme.DiffHighlightRemovedColor = AdaptiveColor{
		Dark:  "#FADADD", // From existing diff.go
		Light: "#EF9A9A", // Light equivalent
	}
	theme.DiffAddedBgColor = AdaptiveColor{
		Dark:  "#303A30", // From existing diff.go
		Light: "#E8F5E9", // Light equivalent
	}
	theme.DiffRemovedBgColor = AdaptiveColor{
		Dark:  "#3A3030", // From existing diff.go
		Light: "#FFEBEE", // Light equivalent
	}
	theme.DiffContextBgColor = AdaptiveColor{
		Dark:  "#212121", // From existing diff.go
		Light: "#F5F5F5", // Light equivalent
	}
	theme.DiffLineNumberColor = AdaptiveColor{
		Dark:  "#888888", // From existing diff.go
		Light: "#9E9E9E", // Light equivalent
	}
	theme.DiffAddedLineNumberBgColor = AdaptiveColor{
		Dark:  "#293229", // From existing diff.go
		Light: "#C8E6C9", // Light equivalent
	}
	theme.DiffRemovedLineNumberBgColor = AdaptiveColor{
		Dark:  "#332929", // From existing diff.go
		Light: "#FFCDD2", // Light equivalent
	}

	// Markdown colors
	theme.MarkdownTextColor = AdaptiveColor{
		Dark:  mocha.Text().Hex,
		Light: latte.Text().Hex,
	}
	theme.MarkdownHeadingColor = AdaptiveColor{
		Dark:  mocha.Mauve().Hex,
		Light: latte.Mauve().Hex,
	}
	theme.MarkdownLinkColor = AdaptiveColor{
		Dark:  mocha.Sky().Hex,
		Light: latte.Sky().Hex,
	}
	theme.MarkdownLinkTextColor = AdaptiveColor{
		Dark:  mocha.Pink().Hex,
		Light: latte.Pink().Hex,
	}
	theme.MarkdownCodeColor = AdaptiveColor{
		Dark:  mocha.Green().Hex,
		Light: latte.Green().Hex,
	}
	theme.MarkdownBlockQuoteColor = AdaptiveColor{
		Dark:  mocha.Yellow().Hex,
		Light: latte.Yellow().Hex,
	}
	theme.MarkdownEmphColor = AdaptiveColor{
		Dark:  mocha.Yellow().Hex,
		Light: latte.Yellow().Hex,
	}
	theme.MarkdownStrongColor = AdaptiveColor{
		Dark:  mocha.Peach().Hex,
		Light: latte.Peach().Hex,
	}
	theme.MarkdownHorizontalRuleColor = AdaptiveColor{
		Dark:  mocha.Overlay0().Hex,
		Light: latte.Overlay0().Hex,
	}
	theme.MarkdownListItemColor = AdaptiveColor{
		Dark:  mocha.Blue().Hex,
		Light: latte.Blue().Hex,
	}
	theme.MarkdownListEnumerationColor = AdaptiveColor{
		Dark:  mocha.Sky().Hex,
		Light: latte.Sky().Hex,
	}
	theme.MarkdownImageColor = AdaptiveColor{
		Dark:  mocha.Sapphire().Hex,
		Light: latte.Sapphire().Hex,
	}
	theme.MarkdownImageTextColor = AdaptiveColor{
		Dark:  mocha.Pink().Hex,
		Light: latte.Pink().Hex,
	}
	theme.MarkdownCodeBlockColor = AdaptiveColor{
		Dark:  mocha.Text().Hex,
		Light: latte.Text().Hex,
	}

	// Syntax highlighting colors
	theme.SyntaxCommentColor = AdaptiveColor{
		Dark:  mocha.Overlay1().Hex,
		Light: latte.Overlay1().Hex,
	}
	theme.SyntaxKeywordColor = AdaptiveColor{
		Dark:  mocha.Pink().Hex,
		Light: latte.Pink().Hex,
	}
	theme.SyntaxFunctionColor = AdaptiveColor{
		Dark:  mocha.Green().Hex,
		Light: latte.Green().Hex,
	}
	theme.SyntaxVariableColor = AdaptiveColor{
		Dark:  mocha.Sky().Hex,
		Light: latte.Sky().Hex,
	}
	theme.SyntaxStringColor = AdaptiveColor{
		Dark:  mocha.Yellow().Hex,
		Light: latte.Yellow().Hex,
	}
	theme.SyntaxNumberColor = AdaptiveColor{
		Dark:  mocha.Teal().Hex,
		Light: latte.Teal().Hex,
	}
	theme.SyntaxTypeColor = AdaptiveColor{
		Dark:  mocha.Sky().Hex,
		Light: latte.Sky().Hex,
	}
	theme.SyntaxOperatorColor = AdaptiveColor{
		Dark:  mocha.Pink().Hex,
		Light: latte.Pink().Hex,
	}
	theme.SyntaxPunctuationColor = AdaptiveColor{
		Dark:  mocha.Text().Hex,
		Light: latte.Text().Hex,
	}

	return theme
}

func init() {
	// Register the Catppuccin theme with the theme manager
	RegisterTheme("catppuccin", NewCatppuccinTheme())
}
