package dialog

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
	"strings"
)

// Command represents an action in the command palette.
type Command struct {
	ID          string
	Title       string
	Description string
	Category    string
	Shortcut    string
	Handler     func(cmd Command) tea.Cmd
}

// CommandSelectedMsg requests execution of the selected action.
type CommandSelectedMsg struct{ Command Command }

// CloseCommandDialogMsg closes the palette without changing the draft.
type CloseCommandDialogMsg struct{}

// CommandDialog displays searchable commands.
type CommandDialog interface {
	util.Model
	layout.Bindings
	SetCommands([]Command)
}
type commandRow struct {
	heading string
	command Command
}
type commandDialogCmp struct {
	search                          textinput.Model
	commands                        []Command
	rows                            []commandRow
	selected, offset, width, height int
}

func (c *commandDialogCmp) Init() tea.Cmd   { return c.search.Focus() }
func (c *commandDialogCmp) listHeight() int { return max(1, min(20, c.height-10)) }
func (c *commandDialogCmp) rebuild() {
	c.rows = nil
	query := strings.Fields(strings.ToLower(c.search.Value()))
	groups := []string{}
	grouped := map[string][]Command{}
	for _, command := range c.commands {
		haystack := strings.ToLower(command.Title + " " + command.Description + " " + command.ID + " " + command.Category)
		matches := true
		for _, word := range query {
			if !strings.Contains(haystack, word) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		category := command.Category
		if category == "" {
			category = "Custom"
		}
		if _, ok := grouped[category]; !ok {
			groups = append(groups, category)
		}
		grouped[category] = append(grouped[category], command)
	}
	for _, group := range groups {
		c.rows = append(c.rows, commandRow{heading: group})
		for _, command := range grouped[group] {
			c.rows = append(c.rows, commandRow{command: command})
		}
	}
	c.selected = -1
	c.offset = 0
	for i, row := range c.rows {
		if row.heading == "" {
			c.selected = i
			break
		}
	}
}
func (c *commandDialogCmp) move(delta int) {
	for i := c.selected + delta; i >= 0 && i < len(c.rows); i += delta {
		if c.rows[i].heading != "" {
			continue
		}
		c.selected = i
		if i < c.offset {
			c.offset = i
		}
		if i >= c.offset+c.listHeight() {
			c.offset = i - c.listHeight() + 1
		}
		return
	}
}
func (c *commandDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width, c.height = msg.Width, msg.Height
		c.search.SetWidth(max(1, min(68, c.width-8)))
		c.offset = max(0, min(c.offset, len(c.rows)-c.listHeight()))
		if c.selected >= c.offset+c.listHeight() {
			c.offset = c.selected - c.listHeight() + 1
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+p", "ctrl+k", "f1":
			return c, util.CmdHandler(CloseCommandDialogMsg{})
		case "up", "ctrl+n":
			if msg.String() == "ctrl+n" {
				c.move(1)
			} else {
				c.move(-1)
			}
			return c, nil
		case "down":
			c.move(1)
			return c, nil
		case "enter":
			if c.selected >= 0 && c.selected < len(c.rows) {
				return c, util.CmdHandler(CommandSelectedMsg{Command: c.rows[c.selected].command})
			}
			return c, nil
		}
	}
	before := c.search.Value()
	var cmd tea.Cmd
	c.search, cmd = c.search.Update(msg)
	if before != c.search.Value() {
		c.rebuild()
	}
	return c, cmd
}
func (c *commandDialogCmp) View() string {
	t := theme.CurrentTheme()
	width := max(1, min(76, c.width-2))
	inner := max(1, width-4)
	base := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.Text())
	muted := base.Foreground(t.TextMuted())
	inputStyles := c.search.Styles()
	inputStyles.Focused.Text = base
	inputStyles.Focused.Prompt = base
	inputStyles.Focused.Placeholder = muted
	c.search.SetStyles(inputStyles)
	line := func(s string) string { return base.Width(inner).Render(ansi.Truncate(s, inner, "…")) }
	title := base.Bold(true).Render("Commands")
	title += strings.Repeat(" ", max(1, inner-lipgloss.Width(title)-3)) + muted.Render("esc")
	lines := []string{line(title), line(""), line(c.search.View()), line("")}
	end := min(len(c.rows), c.offset+c.listHeight())
	for i := c.offset; i < end; i++ {
		row := c.rows[i]
		if row.heading != "" {
			lines = append(lines, line(base.Foreground(t.Primary()).Bold(true).Render(" "+row.heading)))
			continue
		}
		shortcut := row.command.Shortcut
		if lipgloss.Width(shortcut) > inner/2 {
			shortcut = ""
		}
		name := ansi.Truncate(" "+row.command.Title, max(1, inner-lipgloss.Width(shortcut)-2), "…")
		gap := strings.Repeat(" ", max(1, inner-lipgloss.Width(name)-lipgloss.Width(shortcut)-1))
		if i == c.selected {
			lines = append(lines, base.Background(t.Primary()).Foreground(t.Background()).Bold(true).Width(inner).Render(name+gap+shortcut+" "))
		} else {
			lines = append(lines, line(base.Render(name+gap)+muted.Render(shortcut+" ")))
		}
	}
	if len(c.rows) == 0 {
		lines = append(lines, line(muted.Render("No matching commands")))
	}
	lines = append(lines, line(""), line(muted.Render("↑↓ select · enter run · esc close")))
	view := base.Width(width).Padding(1, 2).Render(strings.Join(lines, "\n"))
	return lipgloss.NewStyle().MaxWidth(max(1, c.width)).MaxHeight(max(1, c.height)).Render(styles.Surface(view, t.BackgroundSecondary()))
}
func (c *commandDialogCmp) BindingKeys() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "select")), key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")), key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))}
}
func (c *commandDialogCmp) SetCommands(commands []Command) {
	c.commands = append([]Command(nil), commands...)
	c.search.SetValue("")
	c.rebuild()
}

// NewCommandDialogCmp creates the searchable command palette.
func NewCommandDialogCmp() CommandDialog {
	search := textinput.New()
	search.Placeholder = "Search commands…"
	search.Prompt = ""
	search.CharLimit = 128
	return &commandDialogCmp{search: search, width: 80, height: 30, selected: -1}
}
