package styles

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestSurfaceRestoresBackgroundAfterNestedReset(t *testing.T) {
	content := "\x1b[31mtext\x1b[0m    \n  \x1b[49m padding"
	view := Surface(content, lipgloss.Color("#292929"))
	sample := lipgloss.NewStyle().Background(lipgloss.Color("#292929")).Render(" ")
	bg := sample[:strings.IndexByte(sample, 'm')+1]
	require.Contains(t, view, "\x1b[31mtext")
	require.Contains(t, view, "\x1b[0m"+bg+"    ")
	require.Contains(t, view, bg+" padding")
	require.True(t, strings.HasSuffix(view, "\x1b[0m"))
}
