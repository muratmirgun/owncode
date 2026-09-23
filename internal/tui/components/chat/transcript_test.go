package chat

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestVirtualTranscriptMatchesVisibleRows(t *testing.T) {
	v := newTranscriptViewport()
	v.SetWidth(20)
	v.SetHeight(4)
	items := []uiMessage{{content: "one\ntwo"}, {content: "\x1b[31mred\x1b[0m\n四\nfive\nsix"}}
	v.SetItems(items)
	reference := viewport.New(viewport.WithWidth(20), viewport.WithHeight(4))
	reference.SetContent(v.GetContent())
	for offset := 0; offset <= v.maxOffset(); offset++ {
		v.offset = offset
		reference.SetYOffset(offset)
		require.Equal(t, ansi.Strip(reference.View()), ansi.Strip(v.View()))
	}
	before := &v.items[0].lines[0]
	items[1].content += "\nnew"
	v.SetItems(items)
	require.Same(t, before, &v.items[0].lines[0])
}

func TestVirtualTranscriptBoundsLargeItem(t *testing.T) {
	v := newTranscriptViewport()
	v.SetWidth(80)
	v.SetHeight(10)
	v.SetItems([]uiMessage{{content: strings.Repeat("line\n", 100000)}})
	v.GotoBottom()
	require.Less(t, len(v.View()), 2000)
	v.SetItems(nil)
	require.Zero(t, v.YOffset())
	require.True(t, v.AtBottom())
}

func TestTranscriptFrameCacheInvalidates(t *testing.T) {
	v := newTranscriptViewport()
	v.SetWidth(30)
	v.SetHeight(2)
	v.SetItems([]uiMessage{{content: "first\nsecond\nthird"}})
	first := v.View()
	require.True(t, v.frameValid)
	require.Equal(t, first, v.View())
	v.offset = 1
	require.NotEqual(t, first, v.View())
	v.SetItems([]uiMessage{{content: "changed\nsecond\nthird"}})
	require.False(t, v.frameValid)
	v.offset = 0
	require.Contains(t, v.View(), "changed")
	v.SetWidth(4)
	require.LessOrEqual(t, lipgloss.Width(v.View()), 4)
	v.SetHeight(1)
	require.Equal(t, 1, lipgloss.Height(v.View()))
}
