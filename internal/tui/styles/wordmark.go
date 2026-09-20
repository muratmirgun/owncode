package styles

import (
	"charm.land/lipgloss/v2"
	"strings"
)

// Wordmark renders the shared ANSI Shadow logo without a background.
func Wordmark(width int, ownStyle, codeStyle lipgloss.Style) string {
	own := []string{
		" ██████╗ ██╗    ██╗███╗   ██╗",
		"██╔═══██╗██║    ██║████╗  ██║",
		"██║   ██║██║ █╗ ██║██╔██╗ ██║",
		"██║   ██║██║███╗██║██║╚██╗██║",
		"╚██████╔╝╚███╔███╔╝██║ ╚████║",
		" ╚═════╝  ╚══╝╚══╝ ╚═╝  ╚═══╝",
	}
	code := []string{
		" ██████╗ ██████╗ ██████╗ ███████╗",
		"██╔════╝██╔═══██╗██╔══██╗██╔════╝",
		"██║     ██║   ██║██║  ██║█████╗  ",
		"██║     ██║   ██║██║  ██║██╔══╝  ",
		"╚██████╗╚██████╔╝██████╔╝███████╗",
		" ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝",
	}
	if width < lipgloss.Width(own[0]+code[0]) {
		return ownStyle.Render("Own") + codeStyle.Render("Code")
	}
	lines := make([]string, len(own))
	for i := range own {
		lines[i] = ownStyle.Render(own[i]) + codeStyle.Render(code[i])
	}
	return strings.Join(lines, "\n")
}
