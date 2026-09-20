package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/permission"
	"github.com/stretchr/testify/require"
)

func TestDockedPermissionKeepsActions(t *testing.T) {
	p := NewPermissionDialogCmp()
	p.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	request := permission.PermissionRequest{ID: "test", ToolName: tools.BashToolName, Path: "/tmp", Params: tools.BashPermissionsParams{Command: "go test ./..."}}
	p.SetPermissions(request)
	view := p.View()
	require.Equal(t, 100, lipgloss.Width(view))
	require.LessOrEqual(t, lipgloss.Height(view), 14)
	require.Contains(t, view, "Permission required")
	require.Contains(t, view, "Allow [a]")
	require.Contains(t, view, "v details")
	require.Contains(t, ansi.Strip(view), "go test ./...")
	require.LessOrEqual(t, lipgloss.Height(view), 12)
	p.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	require.Greater(t, lipgloss.Height(p.View()), lipgloss.Height(view))
	p.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, selected := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, PermissionAllowForSession, selected().(PermissionResponseMsg).Action)
	require.Contains(t, ansi.Strip(p.View()), "› 2. Allow for session")
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, selected = p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, PermissionDeny, selected().(PermissionResponseMsg).Action)
	for _, test := range []struct {
		key    string
		action PermissionAction
	}{{"a", PermissionAllow}, {"s", PermissionAllowForSession}, {"d", PermissionDeny}} {
		_, cmd := p.Update(tea.KeyPressMsg{Code: []rune(test.key)[0], Text: test.key})
		response := cmd().(PermissionResponseMsg)
		require.Equal(t, test.action, response.Action)
		require.Equal(t, request, response.Permission)
	}
	p.Update(tea.WindowSizeMsg{Width: 35, Height: 24})
	require.Equal(t, 35, lipgloss.Width(p.View()))
	require.False(t, strings.Contains(p.View(), "░"))
}
