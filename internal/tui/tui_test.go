package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestDialogInputClassification(t *testing.T) {
	require.True(t, isInputMessage(tea.PasteMsg{Content: "draft"}))
	require.True(t, isInputMessage(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.True(t, isInputMessage(tea.KeyReleaseMsg{Code: tea.KeyEnter}))
	require.False(t, isInputMessage(tea.WindowSizeMsg{Width: 80, Height: 24}))
}
