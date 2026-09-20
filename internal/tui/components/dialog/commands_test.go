package dialog

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestPaletteSearchAndSelection(t *testing.T) {
	c := NewCommandDialogCmp().(*commandDialogCmp)
	c.SetCommands([]Command{{ID: "sessions", Title: "Switch session", Category: "Suggested"}, {ID: "models", Title: "Switch model", Category: "Suggested"}, {ID: "settings", Title: "Open settings", Category: "System"}})
	c.Init()
	c.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if c.rows[c.selected].command.ID != "models" {
		t.Fatal("navigation selected a heading")
	}
	c.Update(tea.PasteMsg{Content: "settings"})
	if len(c.rows) != 2 || c.rows[c.selected].command.ID != "settings" {
		t.Fatal("search did not filter commands")
	}
	_, cmd := c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || cmd().(CommandSelectedMsg).Command.ID != "settings" {
		t.Fatal("enter did not select filtered action")
	}
	c.Update(tea.PasteMsg{Content: "unknown"})
	_, cmd = c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("empty search executed a command")
	}
	c.SetCommands([]Command{{ID: "new", Title: "New session"}})
	if c.search.Value() != "" || c.selected < 0 {
		t.Fatal("reopening retained search")
	}
}

func TestPaletteScrollAndSmallTerminal(t *testing.T) {
	c := NewCommandDialogCmp().(*commandDialogCmp)
	commands := []Command{}
	for i := range 40 {
		commands = append(commands, Command{ID: fmt.Sprint(i), Title: fmt.Sprintf("Command %d", i), Category: "Custom", Shortcut: "ctrl+x"})
	}
	c.SetCommands(commands)
	c.Update(tea.WindowSizeMsg{Width: 42, Height: 16})
	for range 39 {
		c.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if c.rows[c.selected].command.ID != "39" || c.offset == 0 {
		t.Fatal("selection did not scroll")
	}
	view := ansi.Strip(c.View())
	if !strings.Contains(view, "Command 39") || lipgloss.Width(view) > 42 || lipgloss.Height(view) > 16 {
		t.Fatalf("invalid bounded view %q", view)
	}
	_, cmd := c.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if _, ok := cmd().(CloseCommandDialogMsg); !ok {
		t.Fatal("ctrl+p did not close palette")
	}
}
