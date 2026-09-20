package dialog

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/muratmirgun/owncode/internal/diff"
	"github.com/muratmirgun/owncode/internal/recovery"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// CloseRecoveryMsg leaves the current files and conversation intact.
type CloseRecoveryMsg struct{}

// RecoveryAppliedMsg requests a fresh transcript after restoring a turn.
type RecoveryAppliedMsg struct{ Session string }
type recoveryResultMsg struct {
	Preview recovery.Preview
	Content string
	Applied bool
	Err     error
}
type recoveryCmp struct {
	service            *recovery.Service
	session, direction string
	preview            recovery.Preview
	viewport           viewport.Model
	width, height      int
	busy, ready        bool
	notice             string
	previewText        string
}

// NewRecoveryCmp previews all file changes before a turn is restored.
func NewRecoveryCmp(service *recovery.Service, session, direction string) util.Model {
	return &recoveryCmp{service: service, session: session, direction: direction, viewport: viewport.New(), busy: true}
}
func (r *recoveryCmp) Init() tea.Cmd {
	service, session, direction := r.service, r.session, r.direction
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		p, err := service.Inspect(ctx, session, direction)
		text := ""
		if err == nil {
			text = recoveryPreviewContent(p)
		}
		return recoveryResultMsg{Preview: p, Content: text, Err: err}
	}
}
func recoveryPreviewContent(p recovery.Preview) string {
	rows := []string{"Restore the " + p.Direction + " checkpoint?", "Later edits to affected files will block restoration.", "Review changes made by other processes during this turn too.", ""}
	total := 0
	for _, path := range p.Paths {
		before, after := p.Diff(path)
		displayPath := strings.Join(strings.Fields(ansi.Strip(path)), " ")
		if !utf8.ValidString(before) || !utf8.ValidString(after) || strings.ContainsRune(before, 0) || strings.ContainsRune(after, 0) {
			rows = append(rows, fmt.Sprintf("%s · binary file (%d → %d bytes); inspect it before restoring", displayPath, len(before), len(after)))
			continue
		}
		if len(before)+len(after) > 128<<10 {
			rows = append(rows, fmt.Sprintf("%s · large file (%d → %d bytes); inspect it on disk before restoring", displayPath, len(before), len(after)))
			continue
		}
		patch, _, _ := diff.GenerateDiff(before, after, path)
		total += len(patch)
		if total > 64<<10 {
			rows = append(rows, "Preview limit reached. Inspect the remaining paths on disk:", ansi.Strip(strings.Join(p.Paths, "\n")))
			break
		}
		rows = append(rows, displayPath, ansi.Strip(patch), "")
	}
	if len(p.Paths) == 0 {
		rows = append(rows, "No file changes. Restore conversation state only.")
	}
	return strings.Join(rows, "\n")
}
func (r *recoveryCmp) content() {
	r.viewport.SetContent(ansi.Wrap(r.previewText, max(1, r.viewport.Width()), ""))
}

func (r *recoveryCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
		r.viewport.SetWidth(max(1, min(100, r.width-6)))
		r.viewport.SetHeight(max(1, min(28, r.height-10)))
		if r.ready {
			r.content()
		}
	case recoveryResultMsg:
		r.busy = false
		if msg.Err != nil {
			r.notice = msg.Err.Error()
			return r, nil
		}
		if msg.Applied {
			return r, util.CmdHandler(RecoveryAppliedMsg{Session: r.session})
		}
		r.preview = msg.Preview
		r.previewText = msg.Content
		r.ready = true
		r.content()
	case tea.KeyPressMsg:
		if r.busy {
			return r, nil
		}
		if msg.String() == "esc" {
			return r, util.CmdHandler(CloseRecoveryMsg{})
		}
		if msg.String() == "enter" && r.ready {
			r.busy = true
			r.notice = "Restoring…"
			service, p := r.service, r.preview
			return r, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				err := service.Apply(ctx, p)
				return recoveryResultMsg{Applied: err == nil, Err: err}
			}
		}
	}
	var cmd tea.Cmd
	r.viewport, cmd = r.viewport.Update(msg)
	return r, cmd
}
func (r *recoveryCmp) View() string {
	t := theme.CurrentTheme()
	base := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.Text())
	width := max(1, min(104, r.width-2))
	inner := max(1, width-4)
	notice := r.notice
	if r.busy && notice == "" {
		notice = "Loading checkpoint…"
	}
	line := func(text string) string { return base.Width(inner).Render(ansi.Truncate(text, inner, "…")) }
	content := strings.Join([]string{line("Turn recovery · " + r.direction), line(""), r.viewport.View(), line(""), line(ansi.Strip(notice)), line("↑↓ scroll · enter restore · esc cancel")}, "\n")
	return lipgloss.NewStyle().MaxWidth(max(1, r.width)).MaxHeight(max(1, r.height)).Render(styles.Surface(base.Width(width).Padding(1, 2).Render(content), t.BackgroundSecondary()))
}
