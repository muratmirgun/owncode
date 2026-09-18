package dialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muratmirgun/owncode/internal/tui/util"
	"github.com/stretchr/testify/require"
)

func TestEmptyModelDialog(t *testing.T) {
	t.Parallel()
	dialog := &modelDialogCmp{}
	for _, key := range []tea.KeyType{tea.KeyUp, tea.KeyDown, tea.KeyEnter} {
		_, cmd := dialog.Update(tea.KeyMsg{Type: key})
		require.NotNil(t, cmd)
		_, ok := cmd().(util.InfoMsg)
		require.True(t, ok)
	}
	_, cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, ok := cmd().(CloseModelDialogMsg)
	require.True(t, ok)
}
