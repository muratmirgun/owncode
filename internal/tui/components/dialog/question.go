package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	questions "github.com/muratmirgun/owncode/internal/question"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// QuestionReplyMsg returns a UI answer to its originating request.
type QuestionReplyMsg struct {
	ID     string
	Answer questions.Answer
}
type questionCmp struct {
	request         questions.Request
	input           textinput.Model
	selected, width int
	typing          bool
}

// NewQuestionCmp creates an inline choice list with a free-text answer.
func NewQuestionCmp(request questions.Request) util.Model {
	input := textinput.New()
	input.Placeholder = "Type your answer…"
	input.CharLimit = 4096
	return &questionCmp{request: request, input: input, width: 80}
}
func (q *questionCmp) Init() tea.Cmd { return nil }
func (q *questionCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		q.width = msg.Width
		q.input.SetWidth(max(1, msg.Width-8))
	case tea.KeyPressMsg:
		answer := func(value questions.Answer) tea.Cmd {
			return util.CmdHandler(QuestionReplyMsg{ID: q.request.ID, Answer: value})
		}
		if msg.String() == "esc" {
			return q, answer(questions.Answer{Dismissed: true})
		}
		if q.typing {
			if msg.String() == "enter" {
				if strings.TrimSpace(q.input.Value()) != "" {
					return q, answer(questions.Answer{Text: q.input.Value()})
				}
				return q, nil
			}
			var cmd tea.Cmd
			q.input, cmd = q.input.Update(msg)
			return q, cmd
		}
		switch msg.String() {
		case "up":
			q.selected = max(0, q.selected-1)
		case "down", "tab":
			q.selected = (q.selected + 1) % (len(q.request.Options) + 1)
		case "enter":
			if q.selected < len(q.request.Options) {
				return q, answer(questions.Answer{Text: q.request.Options[q.selected]})
			}
			q.typing = true
			return q, q.input.Focus()
		}
	case tea.PasteMsg:
		if q.typing {
			var cmd tea.Cmd
			q.input, cmd = q.input.Update(msg)
			return q, cmd
		}
	}
	return q, nil
}
func (q *questionCmp) View() string {
	t := theme.CurrentTheme()
	width := max(1, q.width-6)
	base := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.Text())
	muted := base.Foreground(t.TextMuted())
	rows := []string{base.Bold(true).Render("Question"), ansi.Wrap(ansi.Strip(q.request.Text), width, ""), ""}
	for i, text := range append(append([]string(nil), q.request.Options...), "Type your own answer") {
		line := fmt.Sprintf("  %d. %s", i+1, ansi.Strip(text))
		if i == q.selected {
			line = base.Foreground(t.Primary()).Bold(true).Render("›" + line[1:])
		}
		rows = append(rows, ansi.Wrap(line, width, ""))
	}
	if q.typing {
		rows = append(rows, q.input.View())
	}
	rows = append(rows, "", muted.Render("↑↓ select · enter answer · esc dismiss"))
	return styles.Surface(base.Width(max(1, q.width)).Padding(1, 2).BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(t.Warning()).Render(strings.Join(rows, "\n")), t.BackgroundSecondary())
}
