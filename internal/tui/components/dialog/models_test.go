package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/tui/util"
	"github.com/stretchr/testify/require"
)

func TestEmptyModelDialog(t *testing.T) {
	t.Parallel()
	dialog := &modelDialogCmp{}
	for _, key := range []rune{tea.KeyUp, tea.KeyDown, tea.KeyEnter} {
		_, cmd := dialog.Update(tea.KeyPressMsg{Code: key})
		require.NotNil(t, cmd)
		_, ok := cmd().(util.InfoMsg)
		require.True(t, ok)
	}
	_, cmd := dialog.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, ok := cmd().(CloseModelDialogMsg)
	require.True(t, ok)
}
