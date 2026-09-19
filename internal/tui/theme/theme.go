package theme

import ()

// Theme defines the interface for all UI themes in the application.
// All colors must be defined as AdaptiveColor to support
// both light and dark terminal backgrounds.
type Theme interface {
	// Base colors
	Primary() AdaptiveColor
	Secondary() AdaptiveColor
	Accent() AdaptiveColor

	// Status colors
	Error() AdaptiveColor
	Warning() AdaptiveColor
	Success() AdaptiveColor
	Info() AdaptiveColor

	// Text colors
	Text() AdaptiveColor
	TextMuted() AdaptiveColor
	TextEmphasized() AdaptiveColor

	// Background colors
	Background() AdaptiveColor
	BackgroundSecondary() AdaptiveColor
	BackgroundDarker() AdaptiveColor

	// Border colors
	BorderNormal() AdaptiveColor
	BorderFocused() AdaptiveColor
	BorderDim() AdaptiveColor

	// Diff view colors
	DiffAdded() AdaptiveColor
	DiffRemoved() AdaptiveColor
	DiffContext() AdaptiveColor
	DiffHunkHeader() AdaptiveColor
	DiffHighlightAdded() AdaptiveColor
	DiffHighlightRemoved() AdaptiveColor
	DiffAddedBg() AdaptiveColor
	DiffRemovedBg() AdaptiveColor
	DiffContextBg() AdaptiveColor
	DiffLineNumber() AdaptiveColor
	DiffAddedLineNumberBg() AdaptiveColor
	DiffRemovedLineNumberBg() AdaptiveColor

	// Markdown colors
	MarkdownText() AdaptiveColor
	MarkdownHeading() AdaptiveColor
	MarkdownLink() AdaptiveColor
	MarkdownLinkText() AdaptiveColor
	MarkdownCode() AdaptiveColor
	MarkdownBlockQuote() AdaptiveColor
	MarkdownEmph() AdaptiveColor
	MarkdownStrong() AdaptiveColor
	MarkdownHorizontalRule() AdaptiveColor
	MarkdownListItem() AdaptiveColor
	MarkdownListEnumeration() AdaptiveColor
	MarkdownImage() AdaptiveColor
	MarkdownImageText() AdaptiveColor
	MarkdownCodeBlock() AdaptiveColor

	// Syntax highlighting colors
	SyntaxComment() AdaptiveColor
	SyntaxKeyword() AdaptiveColor
	SyntaxFunction() AdaptiveColor
	SyntaxVariable() AdaptiveColor
	SyntaxString() AdaptiveColor
	SyntaxNumber() AdaptiveColor
	SyntaxType() AdaptiveColor
	SyntaxOperator() AdaptiveColor
	SyntaxPunctuation() AdaptiveColor
}

// BaseTheme provides a default implementation of the Theme interface
// that can be embedded in concrete theme implementations.
type BaseTheme struct {
	// Base colors
	PrimaryColor   AdaptiveColor
	SecondaryColor AdaptiveColor
	AccentColor    AdaptiveColor

	// Status colors
	ErrorColor   AdaptiveColor
	WarningColor AdaptiveColor
	SuccessColor AdaptiveColor
	InfoColor    AdaptiveColor

	// Text colors
	TextColor           AdaptiveColor
	TextMutedColor      AdaptiveColor
	TextEmphasizedColor AdaptiveColor

	// Background colors
	BackgroundColor          AdaptiveColor
	BackgroundSecondaryColor AdaptiveColor
	BackgroundDarkerColor    AdaptiveColor

	// Border colors
	BorderNormalColor  AdaptiveColor
	BorderFocusedColor AdaptiveColor
	BorderDimColor     AdaptiveColor

	// Diff view colors
	DiffAddedColor               AdaptiveColor
	DiffRemovedColor             AdaptiveColor
	DiffContextColor             AdaptiveColor
	DiffHunkHeaderColor          AdaptiveColor
	DiffHighlightAddedColor      AdaptiveColor
	DiffHighlightRemovedColor    AdaptiveColor
	DiffAddedBgColor             AdaptiveColor
	DiffRemovedBgColor           AdaptiveColor
	DiffContextBgColor           AdaptiveColor
	DiffLineNumberColor          AdaptiveColor
	DiffAddedLineNumberBgColor   AdaptiveColor
	DiffRemovedLineNumberBgColor AdaptiveColor

	// Markdown colors
	MarkdownTextColor            AdaptiveColor
	MarkdownHeadingColor         AdaptiveColor
	MarkdownLinkColor            AdaptiveColor
	MarkdownLinkTextColor        AdaptiveColor
	MarkdownCodeColor            AdaptiveColor
	MarkdownBlockQuoteColor      AdaptiveColor
	MarkdownEmphColor            AdaptiveColor
	MarkdownStrongColor          AdaptiveColor
	MarkdownHorizontalRuleColor  AdaptiveColor
	MarkdownListItemColor        AdaptiveColor
	MarkdownListEnumerationColor AdaptiveColor
	MarkdownImageColor           AdaptiveColor
	MarkdownImageTextColor       AdaptiveColor
	MarkdownCodeBlockColor       AdaptiveColor

	// Syntax highlighting colors
	SyntaxCommentColor     AdaptiveColor
	SyntaxKeywordColor     AdaptiveColor
	SyntaxFunctionColor    AdaptiveColor
	SyntaxVariableColor    AdaptiveColor
	SyntaxStringColor      AdaptiveColor
	SyntaxNumberColor      AdaptiveColor
	SyntaxTypeColor        AdaptiveColor
	SyntaxOperatorColor    AdaptiveColor
	SyntaxPunctuationColor AdaptiveColor
}

// Implement the Theme interface for BaseTheme
func (t *BaseTheme) Primary() AdaptiveColor   { return t.PrimaryColor }
func (t *BaseTheme) Secondary() AdaptiveColor { return t.SecondaryColor }
func (t *BaseTheme) Accent() AdaptiveColor    { return t.AccentColor }

func (t *BaseTheme) Error() AdaptiveColor   { return t.ErrorColor }
func (t *BaseTheme) Warning() AdaptiveColor { return t.WarningColor }
func (t *BaseTheme) Success() AdaptiveColor { return t.SuccessColor }
func (t *BaseTheme) Info() AdaptiveColor    { return t.InfoColor }

func (t *BaseTheme) Text() AdaptiveColor           { return t.TextColor }
func (t *BaseTheme) TextMuted() AdaptiveColor      { return t.TextMutedColor }
func (t *BaseTheme) TextEmphasized() AdaptiveColor { return t.TextEmphasizedColor }

func (t *BaseTheme) Background() AdaptiveColor          { return t.BackgroundColor }
func (t *BaseTheme) BackgroundSecondary() AdaptiveColor { return t.BackgroundSecondaryColor }
func (t *BaseTheme) BackgroundDarker() AdaptiveColor    { return t.BackgroundDarkerColor }

func (t *BaseTheme) BorderNormal() AdaptiveColor  { return t.BorderNormalColor }
func (t *BaseTheme) BorderFocused() AdaptiveColor { return t.BorderFocusedColor }
func (t *BaseTheme) BorderDim() AdaptiveColor     { return t.BorderDimColor }

func (t *BaseTheme) DiffAdded() AdaptiveColor            { return t.DiffAddedColor }
func (t *BaseTheme) DiffRemoved() AdaptiveColor          { return t.DiffRemovedColor }
func (t *BaseTheme) DiffContext() AdaptiveColor          { return t.DiffContextColor }
func (t *BaseTheme) DiffHunkHeader() AdaptiveColor       { return t.DiffHunkHeaderColor }
func (t *BaseTheme) DiffHighlightAdded() AdaptiveColor   { return t.DiffHighlightAddedColor }
func (t *BaseTheme) DiffHighlightRemoved() AdaptiveColor { return t.DiffHighlightRemovedColor }
func (t *BaseTheme) DiffAddedBg() AdaptiveColor          { return t.DiffAddedBgColor }
func (t *BaseTheme) DiffRemovedBg() AdaptiveColor        { return t.DiffRemovedBgColor }
func (t *BaseTheme) DiffContextBg() AdaptiveColor        { return t.DiffContextBgColor }
func (t *BaseTheme) DiffLineNumber() AdaptiveColor       { return t.DiffLineNumberColor }
func (t *BaseTheme) DiffAddedLineNumberBg() AdaptiveColor {
	return t.DiffAddedLineNumberBgColor
}
func (t *BaseTheme) DiffRemovedLineNumberBg() AdaptiveColor {
	return t.DiffRemovedLineNumberBgColor
}

func (t *BaseTheme) MarkdownText() AdaptiveColor       { return t.MarkdownTextColor }
func (t *BaseTheme) MarkdownHeading() AdaptiveColor    { return t.MarkdownHeadingColor }
func (t *BaseTheme) MarkdownLink() AdaptiveColor       { return t.MarkdownLinkColor }
func (t *BaseTheme) MarkdownLinkText() AdaptiveColor   { return t.MarkdownLinkTextColor }
func (t *BaseTheme) MarkdownCode() AdaptiveColor       { return t.MarkdownCodeColor }
func (t *BaseTheme) MarkdownBlockQuote() AdaptiveColor { return t.MarkdownBlockQuoteColor }
func (t *BaseTheme) MarkdownEmph() AdaptiveColor       { return t.MarkdownEmphColor }
func (t *BaseTheme) MarkdownStrong() AdaptiveColor     { return t.MarkdownStrongColor }
func (t *BaseTheme) MarkdownHorizontalRule() AdaptiveColor {
	return t.MarkdownHorizontalRuleColor
}
func (t *BaseTheme) MarkdownListItem() AdaptiveColor { return t.MarkdownListItemColor }
func (t *BaseTheme) MarkdownListEnumeration() AdaptiveColor {
	return t.MarkdownListEnumerationColor
}
func (t *BaseTheme) MarkdownImage() AdaptiveColor     { return t.MarkdownImageColor }
func (t *BaseTheme) MarkdownImageText() AdaptiveColor { return t.MarkdownImageTextColor }
func (t *BaseTheme) MarkdownCodeBlock() AdaptiveColor { return t.MarkdownCodeBlockColor }

func (t *BaseTheme) SyntaxComment() AdaptiveColor     { return t.SyntaxCommentColor }
func (t *BaseTheme) SyntaxKeyword() AdaptiveColor     { return t.SyntaxKeywordColor }
func (t *BaseTheme) SyntaxFunction() AdaptiveColor    { return t.SyntaxFunctionColor }
func (t *BaseTheme) SyntaxVariable() AdaptiveColor    { return t.SyntaxVariableColor }
func (t *BaseTheme) SyntaxString() AdaptiveColor      { return t.SyntaxStringColor }
func (t *BaseTheme) SyntaxNumber() AdaptiveColor      { return t.SyntaxNumberColor }
func (t *BaseTheme) SyntaxType() AdaptiveColor        { return t.SyntaxTypeColor }
func (t *BaseTheme) SyntaxOperator() AdaptiveColor    { return t.SyntaxOperatorColor }
func (t *BaseTheme) SyntaxPunctuation() AdaptiveColor { return t.SyntaxPunctuationColor }
