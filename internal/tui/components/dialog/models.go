package dialog

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/auth"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// ModelSelectedMsg requests a model change.
type ModelSelectedMsg struct{ Model models.Model }

// ModelFocusMsg targets a role-specific selection without changing the chat model.
type ModelFocusMsg struct {
	ID    models.ModelID
	Title string
}

// CloseModelDialogMsg closes the model picker.
type CloseModelDialogMsg struct{}

// ConnectModelProviderMsg opens provider setup from the picker.
type ConnectModelProviderMsg struct{}

// RefreshModelsMsg requests a fresh catalog for the selected provider.
type RefreshModelsMsg struct{ Provider string }

// ModelsRefreshedMsg completes a catalog refresh without changing the selected model.
type ModelsRefreshedMsg struct{ Err error }

// ModelDialog is the searchable model picker.
type ModelDialog interface {
	util.Model
	layout.Bindings
}

type modelRow struct {
	heading string
	model   models.Model
}
type modelDialogCmp struct {
	title                                    string
	refreshing                               bool
	refreshStatus                            string
	catalog                                  []models.Model
	rows                                     []modelRow
	active                                   models.ModelID
	prefs                                    modelPreferences
	search                                   textinput.Model
	selectedIdx, scrollOffset, width, height int
}

func NewModelDialogCmp() ModelDialog {
	search := textinput.New()
	search.Placeholder = "Search models or providers"
	search.Prompt = ""
	search.CharLimit = 200
	search.SetWidth(80)
	search.Focus()
	return &modelDialogCmp{search: search, width: 90, height: 32}
}

func (m *modelDialogCmp) Init() tea.Cmd {
	cfg := config.Get()
	m.active = GetSelectedModel(cfg).ID
	m.loadCatalog()
	var err error
	m.prefs, err = loadModelPreferences()
	m.rebuild(m.active)
	if err != nil {
		return util.ReportWarn("Could not load model preferences: " + err.Error())
	}
	return nil
}

func (m *modelDialogCmp) loadCatalog() {
	cfg := config.Get()
	m.catalog = nil
	for _, model := range models.SupportedModels {
		provider, ok := cfg.Providers[model.Provider]
		if ok && !provider.Disabled {
			if model.Custom && (model.Provider == auth.ChatGPT || model.Provider == auth.Claude) {
				if _, exists := provider.Models[model.APIModel]; !exists {
					continue
				}
			}
			m.catalog = append(m.catalog, model)
		}
	}
	slices.SortFunc(m.catalog, func(a, b models.Model) int {
		if a.Provider != b.Provider {
			return strings.Compare(providerLabel(a.Provider), providerLabel(b.Provider))
		}
		if a.Name != b.Name {
			return strings.Compare(b.Name, a.Name)
		}
		return strings.Compare(string(a.ID), string(b.ID))
	})
}

func (m *modelDialogCmp) rebuild(preferred models.ModelID) {
	m.rows = nil
	seen := make(map[models.ModelID]bool)
	matches := func(model models.Model) bool {
		text := strings.ToLower(model.Name + " " + string(model.ID) + " " + providerLabel(model.Provider))
		for _, word := range strings.Fields(strings.ToLower(m.search.Value())) {
			if !strings.Contains(text, word) {
				return false
			}
		}
		return true
	}
	add := func(heading string, ids []models.ModelID) {
		added := false
		for _, id := range ids {
			for _, model := range m.catalog {
				if model.ID != id || seen[id] || !matches(model) {
					continue
				}
				if !added {
					m.rows = append(m.rows, modelRow{heading: heading})
					added = true
				}
				m.rows = append(m.rows, modelRow{model: model})
				seen[id] = true
			}
		}
	}
	add("Favorites", m.prefs.Favorites)
	recent := append([]models.ModelID{m.active}, m.prefs.Recent...)
	add("Recent", recent)
	lastGroup := ""
	for _, model := range m.catalog {
		if seen[model.ID] || !matches(model) {
			continue
		}
		heading := providerLabel(model.Provider)
		if lastGroup != heading {
			lastGroup = heading
			m.rows = append(m.rows, modelRow{heading: heading})
		}
		m.rows = append(m.rows, modelRow{model: model})
		seen[model.ID] = true
	}
	m.selectedIdx = -1
	for i, row := range m.rows {
		if row.heading != "" {
			continue
		}
		if m.selectedIdx < 0 || row.model.ID == preferred {
			m.selectedIdx = i
		}
		if row.model.ID == preferred {
			break
		}
	}
	m.scrollOffset = 0
	m.ensureVisible()
}

func (m *modelDialogCmp) listHeight() int { return max(1, min(23, m.height-9)) }
func (m *modelDialogCmp) ensureVisible() {
	if m.selectedIdx < m.scrollOffset {
		m.scrollOffset = max(0, m.selectedIdx)
	}
	if m.selectedIdx >= m.scrollOffset+m.listHeight() {
		m.scrollOffset = m.selectedIdx - m.listHeight() + 1
	}
	m.scrollOffset = min(m.scrollOffset, max(0, len(m.rows)-m.listHeight()))
}
func (m *modelDialogCmp) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	for range len(m.rows) {
		m.selectedIdx = (m.selectedIdx + delta + len(m.rows)) % len(m.rows)
		if m.rows[m.selectedIdx].heading == "" {
			break
		}
	}
	m.ensureVisible()
}
func (m *modelDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ModelsRefreshedMsg:
		m.refreshing = false
		if msg.Err != nil {
			m.refreshStatus = msg.Err.Error()
			return m, nil
		}
		preferred := m.active
		if m.selectedIdx >= 0 && m.selectedIdx < len(m.rows) {
			preferred = m.rows[m.selectedIdx].model.ID
		}
		m.loadCatalog()
		m.rebuild(preferred)
		m.refreshStatus = "Models refreshed"
		return m, nil
	case ModelFocusMsg:
		m.active = msg.ID
		m.title = msg.Title
		m.rebuild(msg.ID)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.search.SetWidth(max(1, min(86, m.width-6)))
		m.ensureVisible()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+o", "f2":
			return m, util.CmdHandler(CloseModelDialogMsg{})
		case "ctrl+r", "f7":
			if m.refreshing {
				return m, nil
			}
			if m.selectedIdx < 0 || m.selectedIdx >= len(m.rows) {
				return m, nil
			}
			provider := string(m.rows[m.selectedIdx].model.Provider)
			if provider != auth.ChatGPT && provider != auth.Claude {
				m.refreshStatus = "Refresh is available for connected ChatGPT and Claude accounts"
				return m, nil
			}
			m.refreshing = true
			m.refreshStatus = "Refreshing " + providerLabel(models.ModelProvider(provider)) + "…"
			return m, util.CmdHandler(RefreshModelsMsg{Provider: provider})
		case "ctrl+a", "f5":
			return m, util.CmdHandler(ConnectModelProviderMsg{})
		case "up", "ctrl+p":
			m.move(-1)
			return m, nil
		case "down", "ctrl+n":
			m.move(1)
			return m, nil
		case "pgup", "pgdown":
			delta := 1
			if msg.String() == "pgup" {
				delta = -1
			}
			for range m.listHeight() {
				m.move(delta)
			}
			return m, nil
		case "enter":
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.rows) && m.rows[m.selectedIdx].heading == "" {
				return m, util.CmdHandler(ModelSelectedMsg{Model: m.rows[m.selectedIdx].model})
			}
			return m, nil
		case "ctrl+f", "f6":
			if m.selectedIdx < 0 || m.selectedIdx >= len(m.rows) {
				return m, nil
			}
			id := m.rows[m.selectedIdx].model.ID
			next := m.prefs
			next.Favorites = slices.Clone(m.prefs.Favorites)
			if i := slices.Index(next.Favorites, id); i >= 0 {
				next.Favorites = slices.Delete(next.Favorites, i, i+1)
			} else {
				next.Favorites = append(next.Favorites, id)
			}
			if err := saveModelPreferences(next); err != nil {
				return m, util.ReportError(err)
			}
			m.prefs = next
			m.rebuild(id)
			return m, nil
		}
	}
	before := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if before != m.search.Value() {
		m.rebuild("")
	}
	return m, cmd
}

func (m *modelDialogCmp) View() string {
	t := theme.CurrentTheme()
	width := max(1, min(90, m.width-2))
	inner := max(1, width-4)
	base := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.Text())
	muted := base.Foreground(t.TextMuted())
	inputStyles := m.search.Styles()
	inputStyles.Focused.Text = base
	inputStyles.Focused.Placeholder = muted
	inputStyles.Focused.Prompt = base
	m.search.SetStyles(inputStyles)
	line := func(s string) string { return base.Width(inner).Render(ansi.Truncate(s, inner, "…")) }
	heading := m.title
	if heading == "" {
		heading = "Select model"
	}
	title := base.Bold(true).Render(heading)
	title += strings.Repeat(" ", max(1, inner-lipgloss.Width(title)-3)) + muted.Render("esc")
	lines := []string{line(title), line(""), line(m.search.View()), line("")}
	end := min(len(m.rows), m.scrollOffset+m.listHeight())
	for i := m.scrollOffset; i < end; i++ {
		row := m.rows[i]
		if row.heading != "" {
			lines = append(lines, line(base.Foreground(t.Primary()).Bold(true).Render("  "+row.heading)))
			continue
		}
		marker := "  "
		if row.model.ID == m.active {
			marker = "● "
		}
		star := ""
		if slices.Contains(m.prefs.Favorites, row.model.ID) {
			star = " ★"
		}
		name := marker + row.model.Name + star
		provider := "  " + providerLabel(row.model.Provider)
		if i == m.selectedIdx {
			lines = append(lines, base.Background(t.Primary()).Foreground(t.Background()).Bold(true).Width(inner).Render(ansi.Truncate(name+provider, inner, "…")))
		} else {
			lines = append(lines, line(base.Render(name)+muted.Render(provider)))
		}
	}
	if len(m.rows) == 0 {
		label := "No matching models"
		if len(m.catalog) == 0 {
			label = "No models connected · ctrl+a to connect"
		}
		lines = append(lines, line(muted.Render(label)))
	}
	for len(lines) < 4+m.listHeight() {
		lines = append(lines, line(""))
	}
	position := ""
	if len(m.rows) > m.listHeight() {
		position = fmt.Sprintf(" · %d/%d", m.scrollOffset+1, len(m.rows))
	}
	status := "↑↓ select · enter confirm" + position
	if m.refreshStatus != "" {
		status = m.refreshStatus
	}
	lines = append(lines, line(""), line(muted.Render(status)), line(base.Render("Connect provider ")+muted.Render("F5")+base.Render("  Favorite ")+muted.Render("F6")+base.Render("  Refresh ")+muted.Render("F7 / ctrl+r")))
	view := base.Width(width).Padding(1, 2).Render(strings.Join(lines, "\n"))
	view = styles.Surface(view, t.BackgroundSecondary())
	// Keep even very small terminals within their available canvas.
	return lipgloss.NewStyle().MaxWidth(max(1, m.width)).MaxHeight(max(1, m.height)).Render(view)
}

func (m *modelDialogCmp) BindingKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("ctrl+r", "f7"), key.WithHelp("f7 / ctrl+r", "refresh models")),
		key.NewBinding(key.WithKeys("ctrl+a", "f5"), key.WithHelp("f5 / ctrl+a", "connect provider")),
		key.NewBinding(key.WithKeys("ctrl+f", "f6"), key.WithHelp("f6 / ctrl+f", "favorite")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	}
}

func providerLabel(provider models.ModelProvider) string {
	switch provider {
	case "chatgpt":
		return "OpenAI · ChatGPT"
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Claude"
	case "openrouter":
		return "OpenRouter"
	case "xai":
		return "xAI"
	}
	name := []rune(string(provider))
	if len(name) == 0 {
		return "Unknown"
	}
	return strings.ToUpper(string(name[:1])) + string(name[1:])
}

func GetSelectedModel(cfg *config.Config) models.Model {

	agentCfg := cfg.Agents[config.AgentCoder]
	selectedModelId := agentCfg.Model
	return models.SupportedModels[selectedModelId]
}
