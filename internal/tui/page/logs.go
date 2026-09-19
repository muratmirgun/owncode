package page

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/muratmirgun/owncode/internal/tui/components/logs"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

var LogsPage PageID = "logs"

type LogPage interface {
	util.Model
	layout.Sizeable
	layout.Bindings
}
type logsPage struct {
	width, height int
	table         layout.Container
	details       layout.Container
}

func (p *logsPage) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		return p, p.SetSize(msg.Width, msg.Height)
	}

	table, cmd := p.table.Update(msg)
	cmds = append(cmds, cmd)
	p.table = table.(layout.Container)
	details, cmd := p.details.Update(msg)
	cmds = append(cmds, cmd)
	p.details = details.(layout.Container)

	return p, tea.Batch(cmds...)
}

func (p *logsPage) View() string {
	style := styles.BaseStyle().Width(p.width).Height(p.height)
	return style.Render(lipgloss.JoinVertical(lipgloss.Top,
		p.table.View(),
		p.details.View(),
	))
}

func (p *logsPage) BindingKeys() []key.Binding {
	return p.table.BindingKeys()
}

// GetSize implements LogPage.
func (p *logsPage) GetSize() (int, int) {
	return p.width, p.height
}

// SetSize implements LogPage.
func (p *logsPage) SetSize(width int, height int) tea.Cmd {
	p.width = width
	p.height = height
	return tea.Batch(
		p.table.SetSize(width, height/2),
		p.details.SetSize(width, height/2),
	)
}

func (p *logsPage) Init() tea.Cmd {
	return tea.Batch(
		p.table.Init(),
		p.details.Init(),
	)
}

func NewLogsPage() LogPage {
	return &logsPage{
		table:   layout.NewContainer(logs.NewLogsTable(), layout.WithBorderAll()),
		details: layout.NewContainer(logs.NewLogsDetails(), layout.WithBorderAll()),
	}
}
