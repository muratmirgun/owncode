package dialog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type CloseAgentsMsg struct{}
type AgentParentMsg string
type agentRefreshMsg struct{ generation int }
type agentRowsMsg struct {
	parent     string
	generation int
	rows       []agentRow
	err        error
}
type agentRow struct {
	id, title, state, detail string
	tokens                   int64
	cost                     float64
}
type agentsCmp struct {
	app                     *app.App
	parent                  string
	generation              int
	width, height, selected int
	rows                    []agentRow
	details                 bool
	viewport                viewport.Model
	err                     error
}

func NewAgentsCmp(app *app.App) util.Model {
	return &agentsCmp{app: app, viewport: viewport.New()}
}

func (a *agentsCmp) Init() tea.Cmd { return nil }

func (a *agentsCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.viewport.SetWidth(max(1, min(94, a.width-4)-6))
		a.viewport.SetHeight(max(1, min(30, a.height-2)-8))
	case AgentParentMsg:
		a.parent = string(msg)
		a.generation++
		a.selected = 0
		a.rows = nil
		a.details = false
		a.err = nil
		return a, a.load()
	case agentRefreshMsg:
		if msg.generation != a.generation {
			return a, nil
		}
		return a, a.load()
	case agentRowsMsg:
		if msg.parent != a.parent || msg.generation != a.generation {
			return a, nil
		}
		a.rows, a.err = msg.rows, msg.err
		a.selected = min(a.selected, max(0, len(a.rows)-1))
		if a.details && len(a.rows) > 0 {
			a.viewport.SetContent(ansi.Wrap(a.rows[a.selected].detail, a.viewport.Width(), ""))
		}
		generation := a.generation
		return a, tea.Tick(time.Second, func(time.Time) tea.Msg { return agentRefreshMsg{generation} })
	case tea.KeyPressMsg:
		if a.details {
			if msg.String() == "esc" || msg.String() == "enter" {
				a.details = false
				return a, nil
			}
			var cmd tea.Cmd
			a.viewport, cmd = a.viewport.Update(msg)
			return a, cmd
		}
		switch msg.String() {
		case "esc":
			return a, util.CmdHandler(CloseAgentsMsg{})
		case "c":
			if len(a.rows) > 0 && agent.CancelTask(a.rows[a.selected].id) {
				return a, util.ReportInfo("Agent cancellation requested")
			}
		case "up":
			a.selected = max(0, a.selected-1)
		case "down":
			a.selected = min(max(0, len(a.rows)-1), a.selected+1)
		case "enter":
			if len(a.rows) > 0 {
				a.details = true
				a.viewport.SetContent(ansi.Wrap(a.rows[a.selected].detail, a.viewport.Width(), ""))
				a.viewport.GotoTop()
			}
		}
	}
	return a, nil
}

func (a *agentsCmp) load() tea.Cmd {
	parent := a.parent
	generation := a.generation
	return func() tea.Msg {
		if parent == "" {
			return agentRowsMsg{generation: generation, parent: parent}
		}
		ctx := context.Background()
		messages, err := a.app.Messages.List(ctx, parent)
		if err != nil {
			return agentRowsMsg{generation: generation, parent: parent, err: err}
		}
		rows := collectAgentRows(messages, agent.IsTaskRunning)
		for i := range rows {
			child, err := a.app.Sessions.Get(ctx, rows[i].id)
			if err != nil {
				continue
			}
			if child.ParentSessionID != parent {
				continue
			}
			rows[i].tokens = child.PromptTokens + child.CompletionTokens
			rows[i].cost = child.Cost
			history, err := a.app.Messages.List(ctx, child.ID)
			if err != nil {
				continue
			}
			if rows[i].state == "Working" && len(history) > 0 {
				last := history[len(history)-1]
				content := last.Content().String()
				if content != "" {
					rows[i].detail += "\n\nLatest activity\n" + content
				}
				calls := last.ToolCalls()
				if len(calls) > 0 {
					rows[i].detail += "\n\nTool: " + calls[len(calls)-1].Name
				}
			}
		}
		return agentRowsMsg{generation: generation, parent: parent, rows: rows}
	}
}

func collectAgentRows(messages []message.Message, running func(string) bool) []agentRow {
	var rows []agentRow
	index := map[string]int{}
	for _, msg := range messages {
		for _, call := range msg.ToolCalls() {
			if call.Name != agent.AgentToolName {
				continue
			}
			if _, ok := index[call.ID]; ok {
				continue
			}
			var params agent.AgentParams
			_ = json.Unmarshal([]byte(call.Input), &params)
			role := params.Role
			if role == "" {
				role = "explore"
			}
			title := role + " · " + strings.Join(strings.Fields(params.Prompt), " ")
			state := "Interrupted"
			if running(call.ID) {
				state = "Working"
			}
			index[call.ID] = len(rows)
			rows = append(rows, agentRow{id: call.ID, title: title, state: state, detail: "Task\n" + params.Prompt})
		}
		for _, part := range msg.Parts {
			result, ok := part.(message.ToolResult)
			if !ok {
				continue
			}
			i, ok := index[result.ToolCallID]
			if !ok {
				continue
			}
			rows[i].state = "Done"
			if result.IsError {
				rows[i].state = "Failed"
			}
			rows[i].detail += "\n\nResult\n" + result.Content
		}
	}
	return rows
}

func (a *agentsCmp) View() string {
	t := theme.CurrentTheme()
	width := max(12, min(94, a.width-4))
	height := max(8, min(30, a.height-2))
	inner := max(1, width-6)
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	line := func(text string) string { return base.Width(inner).Render(ansi.Truncate(text, inner, "…")) }
	title := base.Foreground(t.Text()).Bold(true).Render("Agents")
	subtitle := "Read-only exploration · Results stay linked to this chat"
	body := []string{}
	if a.details && len(a.rows) > 0 {
		title += "  /  " + a.rows[a.selected].state
		body = append(body, a.viewport.View())
	} else {
		visible := max(1, height-8)
		start := max(0, a.selected-visible+1)
		for i := start; i < min(len(a.rows), start+visible); i++ {
			row := a.rows[i]
			color := t.TextMuted()
			switch row.state {
			case "Working":
				color = t.Primary()
			case "Done":
				color = t.Success()
			case "Failed":
				color = t.Error()
			}
			state := base.Foreground(color).Width(12).Render(row.state)
			text := fmt.Sprintf("%s  %s", state, row.title)
			if i == a.selected {
				text = base.Foreground(t.Primary()).Render("› ") + text
			} else {
				text = "  " + text
			}
			body = append(body, line(text))
		}
		if len(a.rows) == 0 {
			body = append(body, line("No agents in this chat yet."), line(""), line("Ask OwnCode to delegate a focused search or review."), line("The agent tool uses a separate context and cannot edit files."))
		}
		if a.err != nil {
			body = []string{line("Could not load agents: " + a.err.Error())}
		}
		if len(a.rows) > 0 {
			row := a.rows[a.selected]
			body = append(body, line(""), line(fmt.Sprintf("%d tokens · $%.4f · enter opens task and result", row.tokens, row.cost)))
		}
	}
	hint := "↑/↓ select · enter details · c cancel task · esc back"
	if a.details {
		hint = "↑/↓ scroll · pgup/pgdown page · esc agents"
	}
	content := lipgloss.JoinVertical(lipgloss.Left, line(title), line(base.Foreground(t.TextMuted()).Render(subtitle)), line(""), strings.Join(body, "\n"), line(""), line(base.Foreground(t.TextMuted()).Render(hint)))
	return styles.Surface(base.Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(t.BorderNormal()).Render(content), t.BackgroundSecondary())
}
