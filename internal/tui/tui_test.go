package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
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

func TestThemePickerOpensWithCurrentWindowSize(t *testing.T) {
	ui := appModel{width: 120, height: 36, themeDialog: dialog.NewThemeDialogCmp()}
	ui.openThemeDialog()
	view := ui.themeDialog.View()
	require.True(t, ui.showThemeDialog)
	require.Contains(t, ansi.Strip(view), "Themes")
	require.Contains(t, ansi.Strip(view), "enter apply")
	require.Greater(t, lipgloss.Width(view), 40)
	require.LessOrEqual(t, lipgloss.Height(view), ui.height)
	ui.width, ui.height = 50, 22
	ui.openThemeDialog()
	view = ui.themeDialog.View()
	require.Contains(t, ansi.Strip(view), "Themes")
	require.LessOrEqual(t, lipgloss.Width(view), ui.width)
}
