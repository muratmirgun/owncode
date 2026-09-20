package layout

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/muratmirgun/owncode/internal/tui/util"
	"github.com/stretchr/testify/require"
	"testing"
)

type paneFixture struct {
	width, height int
	text          string
}

func (p *paneFixture) Init() tea.Cmd                        { return nil }
func (p *paneFixture) Update(tea.Msg) (util.Model, tea.Cmd) { return p, nil }
func (p *paneFixture) View() string {
	return lipgloss.NewStyle().Width(p.width).Height(p.height).Render(p.text)
}
func (p *paneFixture) SetSize(w, h int) tea.Cmd { p.width = w; p.height = h; return nil }
func (p *paneFixture) GetSize() (int, int)      { return p.width, p.height }
func TestSidebarSpansComposerAndPermission(t *testing.T) {
	left, right, bottom := &paneFixture{text: "chat"}, &paneFixture{text: "sidebar"}, &paneFixture{text: "editor"}
	s := NewSplitPane(WithLeftPanel(NewContainer(left)), WithRightPanel(NewContainer(right)), WithBottomPanel(NewContainer(bottom)))
	s.SetSize(100, 30)
	s.SetBottomAccessory(lipgloss.NewStyle().Width(70).Render("Permission required\nAllow / Session / Deny"))
	require.Equal(t, 30, right.height)
	require.Equal(t, 70, bottom.width)
	require.Equal(t, 30, left.height+bottom.height+2)
	require.Equal(t, 100, lipgloss.Width(s.View()))
	require.Equal(t, 30, lipgloss.Height(s.View()))
}
