package chat

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/skills"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// SlashCommandMsg requests a built-in command without sending a model prompt.
type SlashCommandMsg string

// SlashSuggestionsMsg carries the editor's command popup to the chat page.
type SlashSuggestionsMsg struct{ View string }

type slashCommand struct{ name, description string }

var slashCommands = []slashCommand{
	{"undo", "Preview restoration of the last turn"},
	{"redo", "Preview reapplying an undone turn"},
	{"new", "Start a new chat"},
	{"skills", "Browse, install, and load skills"},
	{"settings", "Model and appearance settings"},
	{"connect", "Connect OpenAI or Claude"},
	{"profiles", "Switch build, plan, or a custom profile"},
	{"models", "Choose a model"},
	{"themes", "Choose a theme"},
	{"sessions", "Open a previous chat"},
	{"compact", "Compact using your saved preferences"},
	{"agents", "Tasks, progress, and results"},
	{"help", "Show keyboard shortcuts"},
}

func (m *editorCmp) matchingCommands() []slashCommand {
	value := m.textarea.Value()
	if m.slashDismissed || (!strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "$")) || strings.ContainsAny(value, " \n\t") {
		return nil
	}
	prefix, commands := "/", slashCommands
	if strings.HasPrefix(value, "$") {
		prefix, commands = "$", m.skillCommands
	}
	query := strings.ToLower(strings.TrimPrefix(value, prefix))
	var matches []slashCommand
	for _, command := range commands {
		if strings.HasPrefix(command.name, query) {
			matches = append(matches, command)
		}
	}
	return matches
}

func (m *editorCmp) slashSuggestions() tea.Cmd {
	matches := m.matchingCommands()
	var rows []string
	t := theme.CurrentTheme()
	prefix := "/"
	if strings.HasPrefix(m.textarea.Value(), "$") {
		prefix = "$"
	}
	start := max(0, m.slashIndex-7)
	for i := start; i < min(len(matches), start+8); i++ {
		command := matches[i]
		text := prefix + command.name + "  " + strings.Join(strings.Fields(ansi.Strip(command.description)), " ")
		style := styles.BaseStyle().Foreground(t.TextMuted())
		if i == m.slashIndex {
			style = style.Foreground(t.Primary()).Bold(true)
			text = "› " + text
		} else {
			text = "  " + text
		}
		rows = append(rows, styles.Surface(style.Width(max(1, m.width)).Render(ansi.Truncate(text, max(1, m.width), "…")), t.Background()))
	}
	return util.CmdHandler(SlashSuggestionsMsg{View: strings.Join(rows, "\n")})
}

func (m *editorCmp) handleSlash(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	matches := m.matchingCommands()
	if len(matches) == 0 {
		return false, nil
	}
	m.slashIndex = min(m.slashIndex, len(matches)-1)
	switch msg.String() {
	case "up", "ctrl+p":
		m.slashIndex = (m.slashIndex + len(matches) - 1) % len(matches)
	case "down", "ctrl+n":
		m.slashIndex = (m.slashIndex + 1) % len(matches)
	case "esc":
		m.slashDismissed = true
	case "tab":
		prefix := "/"
		if strings.HasPrefix(m.textarea.Value(), "$") {
			prefix = "$"
		}
		m.textarea.SetValue(prefix + matches[m.slashIndex].name)
		m.slashIndex = 0
	case "enter":
		if strings.HasPrefix(m.textarea.Value(), "$") {
			m.textarea.SetValue("$" + matches[m.slashIndex].name + " ")
			m.textarea.CursorEnd()
			return true, m.slashSuggestions()
		}
		command := matches[m.slashIndex].name
		m.textarea.Reset()
		m.slashIndex = 0
		return true, tea.Sequence(m.slashSuggestions(), util.CmdHandler(SlashCommandMsg(command)))
	default:
		return false, nil
	}
	return true, m.slashSuggestions()
}

type skillNamesMsg struct {
	owner    *editorCmp
	commands []slashCommand
}

func (m *editorCmp) loadSkillNames() tea.Cmd {
	workdir := ""
	if cfg := config.Get(); cfg != nil {
		workdir = cfg.WorkingDir
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		entries, _ := skills.Discover(ctx, skills.Roots(workdir))
		commands := []slashCommand{}
		for _, entry := range entries {
			if !entry.Disabled && (entry.UserInvocable == nil || *entry.UserInvocable) {
				commands = append(commands, slashCommand{entry.Name, entry.Description})
			}
		}
		return skillNamesMsg{owner: m, commands: commands}
	}
}
