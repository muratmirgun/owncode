package theme

// palette assigns colors by meaning so text, tools, diffs, and dialogs agree.
type palette struct {
	background, panel, inset, text, muted, border  string
	primary, secondary, accent, red, yellow, green string
}

var ownDark = palette{background: "#15171B", panel: "#202329", inset: "#101216", text: "#EBEDF2", muted: "#A5ACB8", border: "#49515F", primary: "#F2BC83", secondary: "#8EBBFF", accent: "#C5ABF5", red: "#FF929B", yellow: "#E7C875", green: "#94D8AE"}
var ownLight = palette{background: "#F5F6F8", panel: "#E8EBF0", inset: "#FFFFFF", text: "#202631", muted: "#535F70", border: "#9BA6B6", primary: "#88501F", secondary: "#275C9C", accent: "#7451A2", red: "#AE3048", yellow: "#785D12", green: "#286843"}

func paired(dark, light string) AdaptiveColor { return AdaptiveColor{Dark: dark, Light: light} }

func paletteTheme(dark, light palette) BaseTheme {
	primary, secondary, accent := paired(dark.primary, light.primary), paired(dark.secondary, light.secondary), paired(dark.accent, light.accent)
	text, muted := paired(dark.text, light.text), paired(dark.muted, light.muted)
	bg, panel, inset := paired(dark.background, light.background), paired(dark.panel, light.panel), paired(dark.inset, light.inset)
	red, yellow, green := paired(dark.red, light.red), paired(dark.yellow, light.yellow), paired(dark.green, light.green)
	border := paired(dark.border, light.border)
	return BaseTheme{
		PrimaryColor: primary, SecondaryColor: secondary, AccentColor: accent,
		ErrorColor: red, WarningColor: yellow, SuccessColor: green, InfoColor: secondary,
		TextColor: text, TextMutedColor: muted, TextEmphasizedColor: text,
		BackgroundColor: bg, BackgroundSecondaryColor: panel, BackgroundDarkerColor: inset,
		BorderNormalColor: border, BorderFocusedColor: primary, BorderDimColor: panel,
		DiffAddedColor: green, DiffRemovedColor: red, DiffContextColor: text, DiffHunkHeaderColor: secondary,
		DiffHighlightAddedColor: paired("#284D38", "#B8DBC4"), DiffHighlightRemovedColor: paired("#583039", "#EBC0C9"),
		DiffAddedBgColor: paired("#192E24", "#DFEEE4"), DiffRemovedBgColor: paired("#342027", "#F6E2E6"), DiffContextBgColor: inset,
		DiffLineNumberColor: muted, DiffAddedLineNumberBgColor: paired("#20382B", "#CDDFD3"), DiffRemovedLineNumberBgColor: paired("#422730", "#EACDD4"),
		MarkdownTextColor: text, MarkdownHeadingColor: primary, MarkdownLinkColor: secondary, MarkdownLinkTextColor: secondary,
		MarkdownCodeColor: accent, MarkdownBlockQuoteColor: muted, MarkdownEmphColor: accent, MarkdownStrongColor: text,
		MarkdownHorizontalRuleColor: border, MarkdownListItemColor: primary, MarkdownListEnumerationColor: primary,
		MarkdownImageColor: secondary, MarkdownImageTextColor: secondary, MarkdownCodeBlockColor: text,
		SyntaxCommentColor: muted, SyntaxKeywordColor: accent, SyntaxFunctionColor: secondary, SyntaxVariableColor: text,
		SyntaxStringColor: green, SyntaxNumberColor: primary, SyntaxTypeColor: yellow, SyntaxOperatorColor: secondary, SyntaxPunctuationColor: muted,
	}
}

// Description gives the theme picker a short palette description.
func Description(name string) string {
	switch name {
	case "owncode":
		return "Graphite · warm amber · default"
	case "midnight":
		return "Deep navy · ice blue · cool focus"
	case "ember":
		return "Warm charcoal · copper · soft gold"
	case "grove":
		return "Forest · mint · quiet contrast"
	default:
		return "Adaptive dark and light palette"
	}
}

func init() {
	midnight := paletteTheme(
		palette{background: "#101723", panel: "#1B2636", inset: "#0B111C", text: "#E6EDF7", muted: "#A1B2C8", border: "#445A76", primary: "#88CFFF", secondary: "#A5BAFF", accent: "#D1A9EB", red: "#FF97A6", yellow: "#EBD08F", green: "#8CDBBD"},
		palette{background: "#F1F5FA", panel: "#E2EAF4", inset: "#FFFFFF", text: "#1D3048", muted: "#4F6380", border: "#92A6BF", primary: "#225E91", secondary: "#465FA0", accent: "#794C94", red: "#AE304A", yellow: "#745A16", green: "#24694F"})
	ember := paletteTheme(
		palette{background: "#1C1715", panel: "#2B2320", inset: "#141110", text: "#F2E7DF", muted: "#BCABA0", border: "#655249", primary: "#F0AE83", secondary: "#E3C480", accent: "#DDB0BE", red: "#FF9690", yellow: "#EACA82", green: "#BCD49A"},
		palette{background: "#FAF4EC", panel: "#EFE3D6", inset: "#FFFDF9", text: "#38291F", muted: "#705B4B", border: "#B9A18A", primary: "#8A471F", secondary: "#735914", accent: "#88465D", red: "#A92D30", yellow: "#785510", green: "#476322"})
	grove := paletteTheme(
		palette{background: "#111C19", panel: "#1C2B25", inset: "#0C1511", text: "#E5EFE7", muted: "#A2B8AB", border: "#455F50", primary: "#95D5AE", secondary: "#8ACDD2", accent: "#D1BE8C", red: "#F59D9A", yellow: "#E1C981", green: "#95D5AE"},
		palette{background: "#F2F7F1", panel: "#E2EBDF", inset: "#FCFFFA", text: "#25362B", muted: "#526A57", border: "#95AA98", primary: "#28653D", secondary: "#276875", accent: "#796022", red: "#A63239", yellow: "#755918", green: "#28653D"})
	RegisterTheme("midnight", &midnight)
	RegisterTheme("ember", &ember)
	RegisterTheme("grove", &grove)
}
