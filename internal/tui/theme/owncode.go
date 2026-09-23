package theme

// OwnCodeTheme provides the default graphite and amber palette.
type OwnCodeTheme struct{ BaseTheme }

// NewOwnCodeTheme creates the default adaptive palette.
func NewOwnCodeTheme() *OwnCodeTheme {
	return &OwnCodeTheme{BaseTheme: paletteTheme(ownDark, ownLight)}
}

func init() { RegisterTheme("owncode", NewOwnCodeTheme()) }
