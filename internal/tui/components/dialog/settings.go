package dialog

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	editingFocus  bool
	focusDraft    string
}

var settingsTabs = []string{"Appearance", "Model", "Context", "Connections", "Orchestration"}

func NewSettingsCmp() util.Model     { return &settingsCmp{} }
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
		agent := config.EffectiveCoder()
		model := models.SupportedModels[agent.Model]
		name := model.Name
		if name == "" {
			name = "Not configured"
		}
		rows = []settingRow{
			{"Selection", "Agent profile", func() string { name, _ := config.CurrentProfile(); return name }(), "Build can edit; Plan uses read-only tools. Custom profiles come from config.", "profiles"},
			{"Selection", "Chat model", name, "Select a configured model for the coding agent.", "models"},
			{"Generation", "Reasoning", reasoningLabel(model, agent.ReasoningEffort), "Enter or Alt+R cycles the supported levels. The change applies to new requests.", "reasoning"},
			{"Limits", "Output limit", fmt.Sprintf("%d tokens", agent.MaxTokens), "Configured maximum output per response. Edit the config to change this value.", ""},
			{"Limits", "Context window", fmt.Sprintf("%d tokens", model.ContextWindow), "Context capacity declared by the selected model. Read-only.", ""},
		}
	case 2:
		jevStatus := "Missing · edit config"
		if cfg.Compaction.Jev.APIKey != "" {
			jevStatus = "Configured"
		}
		rows = []settingRow{
			{"Compaction", "Method", cfg.Compaction.EffectiveMethod(), "Enter cycles eligible methods. Capability rows below explain unavailable methods.", "compact-method"},
			{"Compaction", "Jev key", jevStatus, "Set compaction.jev.apiKey in ~/.owncode.json. The key is never displayed here.", ""},
			{"Compaction", "Summary mode", cfg.Compaction.EffectiveMode(), "Balanced: coding context. Brief: essentials. Handoff: structured continuation notes.", "compact-mode"},
			{"Compaction", "Keep in summary", cfg.Compaction.Focus, "Optional focus: paths, decisions, tests, or details to preserve. Enter to edit.", "compact-focus"},
			{"Compaction", "Auto compact", fmt.Sprint(cfg.AutoCompact), "Automatically compact after a turn reaches the selected context threshold.", "compact-auto"},
			{"Compaction", "Trigger at", fmt.Sprintf("%d%%", cfg.Compaction.EffectiveThreshold()), "Percentage of the model context window. Enter cycles 70 / 80 / 90 / 95.", "compact-threshold"},
			{"Workspace", "Directory", cfg.WorkingDir, "Current project directory. Read-only.", ""},
			{"Storage", "Data directory", cfg.Data.Directory, "Session storage location. Read-only.", ""},
		}
		for _, method := range []string{"summary", "shake", "snapcompact", "jev", "native"} {
			value, description := "Available", "Adapter eligible. Remote model support and history compatibility still apply."
			if reason := config.CompactionUnavailable(method); reason != "" {
				value = "Unavailable"
				description = reason
			}
			rows = append(rows, settingRow{"Capabilities", method, value, description, ""})
		}
	case 3:
		rows = append(rows, settingRow{"Providers", "Connect provider", "OpenAI / Claude", "Sign in with ChatGPT or add an Anthropic API key.", "connect"})
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
	case 4:
		active, _ := config.CurrentProfile()
		rows = append(rows, settingRow{"Witch", "Active profile", active, "Choose Witch to use the main chat as the controller. Build and Plan remain available.", "profiles"})
		for _, lane := range config.WitchLanes() {
			selected := config.WitchLaneSettings(lane)
			agent, _ := config.WitchAgent(lane)
			model := models.SupportedModels[agent.Model]
			name := model.Name
			if name == "" {
				name = "Not configured"
			}
			if selected.Model == "" {
				name = "Chat model · " + name
			}
			label := strings.TrimPrefix(lane, "witch-")
			rows = append(rows,
				settingRow{label, "Model", name, "Select this role's model. Selection stays fixed during dispatches.", "witch-model:" + lane},
				settingRow{label, "Reasoning", reasoningLabel(model, agent.ReasoningEffort), "Choose a supported reasoning level for this role. Changes require idle agents.", "witch-reasoning:" + lane},
				settingRow{label, "Use chat model", "Reset override", "Clear this role's model and reasoning overrides.", "witch-reset:" + lane},
			)
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

func (s *settingsCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	if s.editingFocus {
		switch key := msg.(type) {
		case tea.PasteMsg:
			s.focusDraft += strings.Join(strings.Fields(key.Content), " ")
		case tea.KeyPressMsg:
			switch key.String() {
			case "esc":
				s.editingFocus = false
			case "enter":
				settings := config.Get().Compaction
				settings.Focus = strings.TrimSpace(s.focusDraft)
				if err := config.UpdateCompaction(settings, config.Get().AutoCompact); err != nil {
					return s, util.ReportError(err)
				}
				s.editingFocus = false
				return s, util.ReportInfo("Compaction focus saved")
			case "backspace":
				chars := []rune(s.focusDraft)
				if len(chars) > 0 {
					s.focusDraft = string(chars[:len(chars)-1])
				}
			default:
				s.focusDraft += key.Text
			}
		}
		if _, resizing := msg.(tea.WindowSizeMsg); !resizing {
			return s, nil
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case tea.PasteMsg:
		s.query += strings.Join(strings.Fields(msg.Content), " ")
		s.selected = 0
	case tea.KeyPressMsg:
		if msg.String() == "alt+r" {
			return s, util.CmdHandler(SettingsActionMsg("reasoning"))
		}
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
		case "enter", "space":
			if len(rows) > 0 && rows[min(s.selected, len(rows)-1)].action != "" {
				action := rows[min(s.selected, len(rows)-1)].action
				if strings.HasPrefix(action, "compact-") {
					return s, s.changeCompaction(action)
				}
				return s, util.CmdHandler(SettingsActionMsg(action))
			}
		case "backspace":
			chars := []rune(s.query)
			if len(chars) > 0 {
				s.query = string(chars[:len(chars)-1])
				s.selected = 0
			}
		default:
			if msg.Text != "" {
				s.query += msg.Text
				s.selected = 0
			}
		}
	}
	return s, nil
}

func reasoningLabel(model models.Model, saved string) string {
	if level := model.ReasoningLevel(saved); level != "" {
		return level
	}
	return "Not available"
}

func (s *settingsCmp) View() string {
	t := theme.CurrentTheme()
	width := max(20, min(94, s.width-6))
	height := max(12, min(s.height-4, len(s.rows())+13))
	inner := width - 6
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
	visible := max(1, height-13)
	start := max(0, s.selected-visible+1)
	var body []string
	for i := start; i < min(len(rows), start+visible); i++ {
		row := rows[i]
		if s.editingFocus && row.action == "compact-focus" {
			row.value = s.focusDraft + "▏"
		}
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
	hint := "←/→ tabs · ↑/↓ select · enter change · esc back"
	if s.editingFocus {
		hint = "Type what to preserve · enter save · esc cancel"
	}
	content := lipgloss.JoinVertical(lipgloss.Left, line(title), line(strings.Join(tabs, " ")), line(base.Foreground(t.TextMuted()).Render(search)), line(""), strings.Join(body, "\n"), line(""), base.Height(2).Render(ansi.Wrap(description, inner, "")), line(""), line(hint))
	return styles.Surface(base.Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(t.BorderNormal()).BorderBackground(t.BackgroundSecondary()).Render(content), t.BackgroundSecondary())
}

func (s *settingsCmp) changeCompaction(action string) tea.Cmd {
	cfg := config.Get()
	settings, automatic := cfg.Compaction, cfg.AutoCompact
	switch action {
	case "compact-method":
		methods := []string{"summary", "shake", "snapcompact", "jev", "native"}
		for i, method := range methods {
			if method == settings.EffectiveMethod() {
				for offset := 1; offset <= len(methods); offset++ {
					candidate := methods[(i+offset)%len(methods)]
					if config.CompactionUnavailable(candidate) == "" {
						settings.Method = candidate
						break
					}
				}
				break
			}
		}
	case "compact-mode":
		switch settings.EffectiveMode() {
		case "balanced":
			settings.Mode = "brief"
		case "brief":
			settings.Mode = "handoff"
		default:
			settings.Mode = "balanced"
		}
	case "compact-auto":
		automatic = !automatic
	case "compact-focus":
		s.editingFocus = true
		s.focusDraft = settings.Focus
		return nil
	case "compact-threshold":
		switch settings.EffectiveThreshold() {
		case 70:
			settings.Threshold = 80
		case 80:
			settings.Threshold = 90
		case 90:
			settings.Threshold = 95
		default:
			settings.Threshold = 70
		}
	}
	if err := config.UpdateCompaction(settings, automatic); err != nil {
		return util.ReportError(err)
	}
	return util.ReportInfo("Compaction settings saved")
}
