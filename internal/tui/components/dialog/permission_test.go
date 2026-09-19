package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
