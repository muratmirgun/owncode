package dialog

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// ThemeChangedMsg is sent when the theme is changed
type ThemeChangedMsg struct {
	ThemeName string
}

// CloseThemeDialogMsg is sent when the theme dialog is closed
type CloseThemeDialogMsg struct{}

// ThemeDialog interface for the theme switching dialog
type ThemeDialog interface {
	util.Model
	layout.Bindings
}

type themeDialogCmp struct {
	themes       []string
	selectedIdx  int
	width        int
	height       int
	currentTheme string
}

type themeKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
	J      key.Binding
	K      key.Binding
}

var themeKeys = themeKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous theme"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next theme"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select theme"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	J: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "next theme"),
	),
	K: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "previous theme"),
	),
}

func (t *themeDialogCmp) Init() tea.Cmd {
	// Load available themes and update selectedIdx based on current theme
	t.themes = theme.AvailableThemes()
	t.currentTheme = theme.CurrentThemeName()

	// Find the current theme in the list
	for i, name := range t.themes {
		if name == t.currentTheme {
			t.selectedIdx = i
			break
		}
	}

	return nil
}

func (t *themeDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, themeKeys.Up) || key.Matches(msg, themeKeys.K):
			if t.selectedIdx > 0 {
				t.selectedIdx--
			}
			return t, nil
		case key.Matches(msg, themeKeys.Down) || key.Matches(msg, themeKeys.J):
			if t.selectedIdx < len(t.themes)-1 {
				t.selectedIdx++
			}
			return t, nil
		case key.Matches(msg, themeKeys.Enter):
			if len(t.themes) > 0 {
				previousTheme := theme.CurrentThemeName()
				selectedTheme := t.themes[t.selectedIdx]
				if previousTheme == selectedTheme {
					return t, util.CmdHandler(CloseThemeDialogMsg{})
				}
				if err := theme.SetTheme(selectedTheme); err != nil {
					return t, util.ReportError(err)
				}
				return t, util.CmdHandler(ThemeChangedMsg{
					ThemeName: selectedTheme,
				})
			}
		case key.Matches(msg, themeKeys.Escape):
			return t, util.CmdHandler(CloseThemeDialogMsg{})
		}
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
	}
	return t, nil
}

func (t *themeDialogCmp) View() string {
	current := theme.CurrentTheme()
	width := max(1, min(70, t.width-2))
	inner := max(1, width-4)
	base := lipgloss.NewStyle().Background(current.BackgroundSecondary()).Foreground(current.Text())
	line := func(s string) string { return base.Width(inner).Render(ansi.Truncate(s, inner, "…")) }
	lines := []string{line("Themes"), line("")}
	if len(t.themes) == 0 {
		return line("No themes available")
	}
	visible := max(1, min(7, t.height-16))
	start := max(0, t.selectedIdx-visible+1)
	for i := start; i < min(len(t.themes), start+visible); i++ {
		name := t.themes[i]
		marker := "  "
		if name == t.currentTheme {
			marker = "● "
		}
		style := base
		if i == t.selectedIdx {
			style = style.Background(current.Primary()).Foreground(current.Background()).Bold(true)
		}
		lines = append(lines, style.Width(inner).Render(ansi.Truncate(marker+name, inner, "…")))
	}
	if t.height >= 20 {
		name := t.themes[t.selectedIdx]
		preview := theme.GetTheme(name)
		sample := lipgloss.NewStyle().Background(preview.Background()).Foreground(preview.Text())
		sampleLine := func(s string) string { return sample.Width(inner).Render(ansi.Truncate(s, inner, "…")) }
		lines = append(lines, line(""), line(theme.Description(name)), sampleLine(""),
			sampleLine(sample.Foreground(preview.Primary()).Bold(true).Render("Build")+sample.Foreground(preview.TextMuted()).Render(" · Model · medium")),
			sampleLine(sample.Render("Readable text ")+sample.Foreground(preview.TextMuted()).Render("and supporting detail")),
			sampleLine(sample.Foreground(preview.Success()).Render("✓ Completed  ")+sample.Foreground(preview.Warning()).Render("● Waiting  ")+sample.Foreground(preview.Error()).Render("× Failed")),
			lipgloss.NewStyle().Width(inner).Background(preview.BackgroundSecondary()).Foreground(preview.Text()).Render(ansi.Truncate(" › Message…", inner, "…")),
			lipgloss.NewStyle().Width(inner).Background(preview.Primary()).Foreground(preview.Background()).Bold(true).Render(ansi.Truncate(" Selected item", inner, "…")))
	}
	lines = append(lines, line(""), line("↑↓ preview · enter apply · esc cancel"))
	view := styles.PanelFrame(width).Render(strings.Join(lines, "\n"))
	return lipgloss.NewStyle().MaxWidth(max(1, t.width)).MaxHeight(max(1, t.height)).Render(view)
}

func (t *themeDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(themeKeys)
}

// NewThemeDialogCmp creates a new theme switching dialog
func NewThemeDialogCmp() ThemeDialog {
	return &themeDialogCmp{
		themes:       []string{},
		selectedIdx:  0,
		currentTheme: "",
	}
}
