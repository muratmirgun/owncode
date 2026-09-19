package chat

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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
	{"new", "Start a new chat"},
	{"settings", "Model and appearance settings"},
	{"models", "Choose a model"},
	{"themes", "Choose a theme"},
	{"sessions", "Open a previous chat"},
	{"compact", "Summarize this chat"},
	{"help", "Show keyboard shortcuts"},
}

func (m *editorCmp) matchingCommands() []slashCommand {
	value := m.textarea.Value()
	if m.slashDismissed || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, " \n\t") {
		return nil
	}
	query := strings.ToLower(strings.TrimPrefix(value, "/"))
	var matches []slashCommand
	for _, command := range slashCommands {
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
	for i, command := range matches {
		text := "/" + command.name + "  " + command.description
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
		m.textarea.SetValue("/" + matches[m.slashIndex].name)
		m.slashIndex = 0
	case "enter":
		command := matches[m.slashIndex].name
		m.textarea.Reset()
		m.slashIndex = 0
		return true, tea.Sequence(m.slashSuggestions(), util.CmdHandler(SlashCommandMsg(command)))
	default:
		return false, nil
	}
	return true, m.slashSuggestions()
}
