package dialog

import (
	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/config"
	"testing"
)

func TestAutomationSettingsSelection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := config.Load(dir, false); err != nil {
		t.Fatal(err)
	}
	panel := &settingsCmp{tab: 5, width: 100, height: 35}
	rows := panel.rows()
	if len(rows) < 2 || rows[0].action != "automation-browser" || rows[1].action != "automation-computer" {
		t.Fatal("missing backend selectors")
	}
	_, command := panel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if command == nil || command() != SettingsActionMsg("automation-browser") {
		t.Fatal("browser selection did not dispatch")
	}
}
