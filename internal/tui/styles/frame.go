package styles

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// FitFrame clips and pads an already laid out frame without word wrapping it.
// Message renderers own wrapping; outer panels only position terminal cells.
func FitFrame(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	rows := make([]string, 0, height)
	for line := range strings.SplitSeq(content, "\n") {
		if len(rows) == height {
			break
		}
		lineWidth := ansi.StringWidth(line)
		if lineWidth > width {
			line = ansi.Truncate(line, width, "")
			lineWidth = ansi.StringWidth(line)
		}
		rows = append(rows, line+strings.Repeat(" ", width-lineWidth))
	}
	blank := strings.Repeat(" ", width)
	for len(rows) < height {
		rows = append(rows, blank)
	}
	return strings.Join(rows, "\n")
}
