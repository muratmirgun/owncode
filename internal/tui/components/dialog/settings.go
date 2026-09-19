package dialog

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// CloseSettingsMsg closes the settings screen without changing a setting.
type CloseSettingsMsg struct{}

// SettingsActionMsg opens the existing editor for a selected setting.
type SettingsActionMsg string

type settingRow struct{ section, name, value, description, action string }

type settingsCmp struct {
	width, height int
	tab, selected int
	query         string
}

var settingsTabs = []string{"Appearance", "Model", "Context", "Connections"}

func NewSettingsCmp() tea.Model      { return &settingsCmp{} }
func (s *settingsCmp) Init() tea.Cmd { return nil }

func (s *settingsCmp) rows() []settingRow {
	cfg := config.Get()
	var rows []settingRow
	switch s.tab {
	case 0:
		rows = []settingRow{
			{"Theme", "Color theme", theme.CurrentThemeName(), "Choose a color palette. The change applies immediately.", "themes"},
			{"Composer", "Font", "Terminal font", "Your terminal controls the font family and size. Read-only.", ""},
			{"Status", "Throughput", "Live · 200 ms", "Estimated TPS, recent response history, and average speed. Read-only.", ""},
			{"Layout", "Welcome screen", "Centered", "A new chat opens without a sidebar. Active chats show the sidebar. Read-only.", ""},
		}
	case 1:
		agent := cfg.Agents[config.AgentCoder]
		model := models.SupportedModels[agent.Model]
		name := model.Name
		if name == "" {
			name = "Not configured"
		}
		rows = []settingRow{
			{"Selection", "Chat model", name, "Select a configured model for the coding agent.", "models"},
			{"Limits", "Output limit", fmt.Sprintf("%d tokens", agent.MaxTokens), "Configured maximum output per response. Edit the config to change this value.", ""},
			{"Limits", "Context window", fmt.Sprintf("%d tokens", model.ContextWindow), "Context capacity declared by the selected model. Read-only.", ""},
		}
	case 2:
		rows = []settingRow{
			{"Compaction", "Auto compact", fmt.Sprint(cfg.AutoCompact), "Summarize near the context limit. Edit autoCompact in the config to change this value.", ""},
			{"Workspace", "Directory", cfg.WorkingDir, "Current project directory. Read-only.", ""},
			{"Storage", "Data directory", cfg.Data.Directory, "Session storage location. Read-only.", ""},
		}
	case 3:
		var names []string
		for name := range cfg.MCPServers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			rows = append(rows, settingRow{"MCP", name, string(cfg.MCPServers[name].Type), "Configured MCP transport. This is configuration, not a live connection check.", ""})
		}
		names = nil
		for name := range cfg.LSP {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			rows = append(rows, settingRow{"Language servers", name, cfg.LSP[name].Command, "Configured language server command. Read-only.", ""})
		}
	}
	if s.query == "" {
		return rows
	}
	var filtered []settingRow
	for _, row := range rows {
		if strings.Contains(strings.ToLower(row.name+" "+row.section+" "+row.value), strings.ToLower(s.query)) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (s *settingsCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case tea.KeyMsg:
		rows := s.rows()
		switch msg.String() {
		case "esc":
			if s.query != "" {
				s.query = ""
				s.selected = 0
				return s, nil
			}
			return s, util.CmdHandler(CloseSettingsMsg{})
		case "left", "shift+tab":
			s.tab = (s.tab + len(settingsTabs) - 1) % len(settingsTabs)
			s.selected = 0
			s.query = ""
		case "right", "tab":
			s.tab = (s.tab + 1) % len(settingsTabs)
			s.selected = 0
			s.query = ""
		case "up":
			s.selected = max(0, s.selected-1)
		case "down":
			s.selected = min(max(0, len(rows)-1), s.selected+1)
		case "enter", " ":
			if len(rows) > 0 && rows[min(s.selected, len(rows)-1)].action != "" {
				return s, util.CmdHandler(SettingsActionMsg(rows[min(s.selected, len(rows)-1)].action))
			}
		case "backspace":
			chars := []rune(s.query)
			if len(chars) > 0 {
				s.query = string(chars[:len(chars)-1])
				s.selected = 0
			}
		default:
			if msg.Type == tea.KeyRunes {
				s.query += string(msg.Runes)
				s.selected = 0
			}
		}
	}
	return s, nil
}

func (s *settingsCmp) View() string {
	t := theme.CurrentTheme()
	width := max(20, s.width-6)
	height := max(12, s.height-4)
	inner := width - 4
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	line := func(text string) string { return base.Width(inner).Render(ansi.Truncate(text, inner, "…")) }
	title := base.Foreground(t.Text()).Bold(true).Render("Settings")
	var tabs []string
	for i, name := range settingsTabs {
		style := base.Foreground(t.TextMuted()).Padding(0, 1)
		if i == s.tab {
			style = style.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		}
		tabs = append(tabs, style.Render(name))
	}
	search := "Type to search settings"
	if s.query != "" {
		search = "Search: " + s.query
	}
	rows := s.rows()
	visible := max(1, height-9)
	start := max(0, s.selected-visible+1)
	var body []string
	for i := start; i < min(len(rows), start+visible); i++ {
		row := rows[i]
		sectionWidth := min(18, inner/4)
		labelWidth := min(24, inner/3)
		left := base.Foreground(t.TextMuted()).Width(sectionWidth).Render(ansi.Truncate(row.section, sectionWidth, "…"))
		style := base.Foreground(t.Text())
		prefix := "  "
		if i == s.selected {
			style = style.Background(t.Background()).Foreground(t.Primary()).Bold(true)
			prefix = "› "
		}
		name := ansi.Truncate(prefix+row.name, labelWidth, "…")
		value := row.value
		if row.action != "" {
			value += "  ›"
		}
		right := style.Width(inner - sectionWidth - 3).Render(ansi.Truncate(fmt.Sprintf("%-*s %s", labelWidth, name, value), inner-sectionWidth-3, "…"))
		body = append(body, line(left+base.Foreground(t.BorderNormal()).Render(" │ ")+ansi.Truncate(right, inner-sectionWidth-3, "…")))
	}
	if len(body) == 0 {
		body = append(body, line("No matching settings"))
	}
	for len(body) < visible {
		body = append(body, line(""))
	}
	description := "Select a setting to see its description."
	if len(rows) > 0 {
		description = rows[min(s.selected, len(rows)-1)].description
	}
	content := lipgloss.JoinVertical(lipgloss.Left, line(title), line(strings.Join(tabs, " ")), line(base.Foreground(t.TextMuted()).Render(search)), line(""), strings.Join(body, "\n"), line(""), line(base.Foreground(t.TextMuted()).Render(description)), line(""), line("←/→ tabs · ↑/↓ select · enter change · esc back"))
	return styles.Surface(base.Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(t.BorderNormal()).BorderBackground(t.BackgroundSecondary()).Render(content), t.BackgroundSecondary())
}
