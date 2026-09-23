package dialog

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
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
type WorkerFollowupMsg string
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
	metadata                 *message.Message
	tokens                   int64
	latest                   string
	elapsed                  time.Duration
	cost                     float64
}
type agentsCmp struct {
	app                     *app.App
	parent                  string
	generation              int
	width, height, selected int
	rows                    []agentRow
	details                 bool
	composing               bool
	input                   textinput.Model
	viewport                viewport.Model
	err                     error
}

func NewAgentsCmp(app *app.App) util.Model {
	input := textinput.New()
	input.Placeholder = "Follow-up for this worker…"
	input.CharLimit = 16000
	return &agentsCmp{app: app, viewport: viewport.New(), input: input}
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
		selectedID := ""
		if a.selected < len(a.rows) {
			selectedID = a.rows[a.selected].id
		}
		a.rows, a.err = msg.rows, msg.err
		slices.SortStableFunc(a.rows, func(a, b agentRow) int { return agentStateRank(a.state) - agentStateRank(b.state) })
		for i, row := range a.rows {
			if selectedID != "" && row.id == selectedID {
				a.selected = i
				break
			}
		}
		a.selected = min(a.selected, max(0, len(a.rows)-1))
		if a.details && len(a.rows) > 0 {
			a.viewport.SetContent(ansi.Wrap(agentDetail(a.rows[a.selected]), a.viewport.Width(), ""))
		}
		generation := a.generation
		return a, tea.Tick(time.Second, func(time.Time) tea.Msg { return agentRefreshMsg{generation} })
	case tea.KeyPressMsg:
		if a.composing {
			switch msg.String() {
			case "esc":
				a.composing = false
				return a, nil
			case "enter":
				if len(a.rows) == 0 {
					return a, nil
				}
				id := a.rows[a.selected].id
				prompt := strings.TrimSpace(a.input.Value())
				if prompt == "" {
					return a, nil
				}
				if agent.IsTaskRunning(id) {
					if err := agent.SteerTask(id, prompt); err != nil {
						return a, util.ReportError(err)
					}
					a.composing = false
					return a, util.ReportInfo("Follow-up queued after the current worker turn")
				}
				a.composing = false
				// The primary turn records the follow-up and owns cost, cancellation, and delivery.
				content := fmt.Sprintf("Resume worker %q with the agent tool (worker_id) and this follow-up:\n%s", id, prompt)
				return a, util.CmdHandler(WorkerFollowupMsg(content))
			}
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			return a, cmd
		}
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
		case "f":
			if len(a.rows) > 0 {
				a.composing = true
				a.input.SetValue("")
				return a, a.input.Focus()
			}
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
				a.viewport.SetContent(ansi.Wrap(agentDetail(a.rows[a.selected]), a.viewport.Width(), ""))
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		messages, err := a.app.Messages.List(ctx, parent)
		if err != nil {
			return agentRowsMsg{generation: generation, parent: parent, err: err}
		}
		rows := collectAgentRows(messages, agent.IsTaskRunning, a.app.CoderAgent != nil && a.app.CoderAgent.IsSessionBusy(parent))
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
			if child.CreatedAt > 0 {
				rows[i].elapsed = max(0, time.Since(time.Unix(child.CreatedAt, 0)))
			}
			history, err := a.app.Messages.List(ctx, child.ID)
			if err != nil {
				continue
			}
			for j := len(history) - 1; j >= 0; j-- {
				if history[j].Role == message.Assistant {
					last := history[j]
					rows[i].latest = strings.Join(strings.Fields(last.Content().Text), " ")
					if last.IsThinking() {
						rows[i].latest = "Thinking…"
					}
					for _, call := range last.ToolCalls() {
						rows[i].latest = "Tool: " + call.Name
					}
					if last.IsFinished() && !agent.IsTaskRunning(child.ID) && rows[i].state != "Queued" {
						if last.FinishReason() == message.FinishReasonEndTurn && rows[i].state != "Failed" {
							rows[i].state = "Done"
						}
						if child.CreatedAt > 0 {
							rows[i].elapsed = max(0, time.Unix(last.FinishPart().Time, 0).Sub(time.Unix(child.CreatedAt, 0)))
						}
					}
					saved := message.Message{Model: last.Model, ReasoningEffort: last.ReasoningEffort}
					rows[i].metadata = &saved
					break
				}
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

func collectAgentRows(messages []message.Message, running func(string) bool, parentBusy ...bool) []agentRow {
	var rows []agentRow
	index := map[string]int{}
	workers := map[string]int{}
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
			if len(parentBusy) > 0 && parentBusy[0] && call.Execution == "queued" {
				state = "Queued"
			}
			id := agent.TaskID(call)
			if running(id) {
				state = "Working"
			}
			if i, exists := workers[id]; exists {
				index[call.ID] = i
				rows[i].state = state
				rows[i].detail += "\n\nFollow-up\n" + params.Prompt
				continue
			}
			workers[id] = len(rows)
			index[call.ID] = len(rows)
			rows = append(rows, agentRow{id: id, title: title, state: state, detail: "Task\n" + params.Prompt})
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
	counts := map[string]int{}
	for _, row := range a.rows {
		counts[row.state]++
	}
	subtitle := fmt.Sprintf("Working %d · Queued %d · Done %d · Failed %d", counts["Working"], counts["Queued"], counts["Done"], counts["Failed"])
	body := []string{}
	if a.composing {
		return base.Width(width).Padding(1, 2).Render("Worker follow-up\n\n" + a.input.View() + "\n\nenter queue / prepare resume · esc back")
	}
	if a.details && len(a.rows) > 0 {
		title += "  /  " + a.rows[a.selected].state
		body = append(body, a.viewport.View())
	} else {
		visible := max(1, (height-10)/3)
		start := max(0, a.selected-visible+1)
		for i := start; i < min(len(a.rows), start+visible); i++ {
			row := a.rows[i]
			color := t.TextMuted()
			switch row.state {
			case "Working":
				color = t.Primary()
			case "Queued":
				color = t.Warning()
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
			latest := row.latest
			if latest == "" {
				latest = "Waiting for model"
				if row.state == "Queued" {
					latest = "Waiting for a worker slot"
				}
			}
			activity := fmt.Sprintf("  %s · %s", row.elapsed.Round(time.Second), latest)
			body = append(body, line(text), line(base.Foreground(t.TextMuted()).Render("  "+util.WorkerMetadataLine(row.metadata, max(1, inner-2)))), line(base.Foreground(t.TextMuted()).Render(activity)))
		}
		if len(a.rows) == 0 {
			body = append(body, line("No agents in this chat yet."), line(""), line("Ask OwnCode to delegate a focused search or review."), line("Each worker uses a separate context and its assigned permissions."))
		}
		if a.err != nil {
			body = []string{line("Could not load agents: " + a.err.Error())}
		}
		if len(a.rows) > 0 {
			row := a.rows[a.selected]
			body = append(body, line(""), line(fmt.Sprintf("%d tokens · $%.4f · enter opens task and result", row.tokens, row.cost)))
		}
	}
	hint := "↑/↓ select · enter details · c cancel · f follow-up · esc back"
	if a.details {
		hint = "↑/↓ scroll · pgup/pgdown page · esc agents"
	}
	content := lipgloss.JoinVertical(lipgloss.Left, line(title), line(base.Foreground(t.TextMuted()).Render(subtitle)), line(""), strings.Join(body, "\n"), line(""), line(base.Foreground(t.TextMuted()).Render(hint)))
	return styles.Surface(base.Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(t.BorderNormal()).Render(content), t.BackgroundSecondary())
}

func agentDetail(row agentRow) string {
	name, effort := util.WorkerMetadata(row.metadata)
	return "Model: " + name + "\nReasoning: " + effort + "\n\n" + row.detail
}

func agentStateRank(state string) int {
	switch state {
	case "Working":
		return 0
	case "Queued":
		return 1
	case "Failed", "Interrupted":
		return 2
	default:
		return 3
	}
}
