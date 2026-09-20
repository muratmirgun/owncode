package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/stretchr/testify/require"
)

func TestWitchSettingsExposeSeparateModelAndReasoningActions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	_, err := config.Load(dir, false)
	require.NoError(t, err)
	settings := &settingsCmp{tab: 4, width: 120, height: 45}
	rows := settings.rows()
	require.Len(t, rows, 1+3*len(config.WitchLanes()))
	for i, lane := range config.WitchLanes() {
		settings.selected = 1 + 3*i
		_, cmd := settings.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.Equal(t, SettingsActionMsg("witch-model:"+lane), cmd())
		settings.selected++
		_, cmd = settings.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.Equal(t, SettingsActionMsg("witch-reasoning:"+lane), cmd())
	}
	view := ansi.Strip(settings.View())
	require.Contains(t, view, "Orchestration")
	require.Contains(t, view, "Reasoning")
	require.Contains(t, view, "final-reviewer")
}
