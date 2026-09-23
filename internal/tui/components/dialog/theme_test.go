package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/muratmirgun/owncode/internal/tui/theme"
)

func TestThemePreviewDoesNotApplySelection(t *testing.T) {
	d := NewThemeDialogCmp().(*themeDialogCmp)
	d.Init()
	before := theme.CurrentThemeName()
	d.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	view := d.View()
	if !strings.Contains(view, theme.Description(d.themes[d.selectedIdx])) {
		t.Fatal("selected theme preview is missing")
	}
	d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if theme.CurrentThemeName() != before {
		t.Fatal("preview or cancel changed the active theme")
	}
}

func TestThemePickerFitsTerminal(t *testing.T) {
	d := NewThemeDialogCmp().(*themeDialogCmp)
	d.Init()
	for _, size := range [][2]int{{100, 40}, {40, 20}, {20, 10}, {1, 1}} {
		d.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := d.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Errorf("picker exceeds terminal size %v", size)
		}
	}
}
