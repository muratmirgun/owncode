package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/tui/components/chat"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

func (a *appModel) registerPaletteCommands() {
	// Keep project commands and user commands after the common actions.
	existing := a.commands
	a.commands = nil
	for _, item := range []struct{ id, title, category, shortcut string }{
		{"sessions", "Switch session", "Suggested", "F3 / ctrl+s"},
		{"models", "Switch model", "Suggested", "F2 / ctrl+o"},
		{"new", "New session", "Session", "/new"},
		{"agents", "View subagents", "Session", "/agents"},
		{"reasoning", "Change reasoning", "Session", "F4 / alt+r"},
		{"settings", "Open settings", "System", "/settings"},
		{"connect", "Connect provider", "System", "/connect"},
		{"themes", "Switch theme", "System", "ctrl+t"},
		{"help", "Help", "System", "ctrl+?"},
	} {
		id := item.id
		a.RegisterCommand(dialog.Command{ID: id, Title: item.title, Category: item.category, Shortcut: item.shortcut, Handler: func(dialog.Command) tea.Cmd { return util.CmdHandler(chat.SlashCommandMsg(id)) }})
	}
	a.RegisterCommand(dialog.Command{ID: "logs", Title: "View logs", Category: "System", Shortcut: "ctrl+l", Handler: func(dialog.Command) tea.Cmd { return util.CmdHandler(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}) }})
	a.RegisterCommand(dialog.Command{ID: "quit", Title: "Exit OwnCode", Category: "System", Shortcut: "ctrl+c", Handler: func(dialog.Command) tea.Cmd { return util.CmdHandler(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) }})
	for _, command := range existing {
		command.Category = "Project"
		if command.ID == "compact" {
			command.Category = "Session"
			command.Shortcut = "/compact"
		}
		a.RegisterCommand(command)
	}
}
