package dialog

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/stretchr/testify/require"
)

func pickerFixture() *modelDialogCmp {
	m := NewModelDialogCmp().(*modelDialogCmp)
	m.catalog = []models.Model{
		{ID: "gpt", Name: "GPT Test", Provider: "chatgpt"},
		{ID: "qwen", Name: "Qwen Test", Provider: "theykk"},
	}
	m.active = "qwen"
	m.rebuild(m.active)
	return m
}
func TestEmptyModelDialog(t *testing.T) {
	m := NewModelDialogCmp().(*modelDialogCmp)
	m.rebuild("")
	for _, code := range []rune{tea.KeyUp, tea.KeyDown, tea.KeyEnter} {
		_, cmd := m.Update(tea.KeyPressMsg{Code: code})
		require.Nil(t, cmd)
	}
	require.Contains(t, ansi.Strip(m.View()), "No models connected")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.IsType(t, CloseModelDialogMsg{}, cmd())
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	require.IsType(t, ConnectModelProviderMsg{}, cmd())
}
func TestModelSearchAndNavigation(t *testing.T) {
	m := pickerFixture()
	require.Equal(t, models.ModelID("qwen"), m.rows[m.selectedIdx].model.ID)
	m.move(1)
	require.Equal(t, models.ModelID("gpt"), m.rows[m.selectedIdx].model.ID)
	m.Update(tea.PasteMsg{Content: "ChatGPT GPT"})
	require.Len(t, m.rows, 2)
	require.Equal(t, models.ModelID("gpt"), m.rows[m.selectedIdx].model.ID)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, models.ModelID("gpt"), cmd().(ModelSelectedMsg).Model.ID)
	m.search.SetValue("unknown")
	m.rebuild("")
	require.Equal(t, -1, m.selectedIdx)
	require.Contains(t, ansi.Strip(m.View()), "No matching models")
}
func TestModelFavoritesAndRecents(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := pickerFixture()
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	prefs, err := loadModelPreferences()
	require.NoError(t, err)
	require.Equal(t, []models.ModelID{"qwen"}, prefs.Favorites)
	require.Equal(t, "Favorites", m.rows[0].heading)
	for i := range 9 {
		require.NoError(t, RecordRecentModel(models.ModelID(fmt.Sprint(i))))
	}
	require.NoError(t, RecordRecentModel("5"))
	prefs, err = loadModelPreferences()
	require.NoError(t, err)
	require.Len(t, prefs.Recent, 6)
	require.Equal(t, models.ModelID("5"), prefs.Recent[0])
	require.Equal(t, []models.ModelID{"qwen"}, prefs.Favorites)
	m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	prefs, err = loadModelPreferences()
	require.NoError(t, err)
	require.Empty(t, prefs.Favorites)
}
func TestModelPickerBoundsAndScroll(t *testing.T) {
	m := pickerFixture()
	for i := range 50 {
		m.catalog = append(m.catalog, models.Model{ID: models.ModelID(fmt.Sprint(i)), Name: strings.Repeat("Long model ", 20), Provider: "theykk"})
	}
	m.rebuild(m.active)
	for _, size := range [][2]int{{100, 40}, {60, 18}, {20, 8}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for range 30 {
			m.move(1)
		}
		require.GreaterOrEqual(t, m.selectedIdx, m.scrollOffset)
		require.Less(t, m.selectedIdx, m.scrollOffset+m.listHeight())
		view := m.View()
		require.LessOrEqual(t, lipgloss.Width(view), size[0])
		require.LessOrEqual(t, lipgloss.Height(view), size[1])
	}
}

func TestRefreshModelsShortcutAndFailure(t *testing.T) {
	m := pickerFixture()
	m.rebuild("gpt")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	require.Equal(t, RefreshModelsMsg{Provider: "chatgpt"}, cmd())
	require.True(t, m.refreshing)
	require.Contains(t, ansi.Strip(m.View()), "Refreshing")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyF7})
	require.Nil(t, cmd, "duplicate refresh must not start another request")
	m.Update(ModelsRefreshedMsg{Err: fmt.Errorf("connection failed")})
	require.False(t, m.refreshing)
	require.Len(t, m.catalog, 2)
	require.Equal(t, models.ModelID("gpt"), m.rows[m.selectedIdx].model.ID)
	require.Contains(t, ansi.Strip(m.View()), "connection failed")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyF7})
	require.Equal(t, RefreshModelsMsg{Provider: "chatgpt"}, cmd())
}
