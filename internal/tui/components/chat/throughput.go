package chat

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
)

type throughputTickMsg time.Time

type throughputSample struct {
	at     time.Time
	tokens int
}

func throughputTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(now time.Time) tea.Msg { return throughputTickMsg(now) })
}

type throughputStats struct {
	created   time.Time
	samples   []throughputSample
	messageID string
	started   time.Time
	tokens    int
	rate      float64
	history   []float64
	total     float64
	count     int
	finished  bool
}

// observe estimates tokens from text, reasoning, and tool arguments. The UI marks
// all values as approximate because providers do not report live token counts.
func (s *throughputStats) observe(msg message.Message, now time.Time) {
	if msg.ID != s.messageID {
		if msg.IsFinished() {
			return
		} // Do not invent timings for loaded messages.
		s.messageID = msg.ID
		s.created = now
		s.samples = nil
		s.started = time.Time{}
		s.tokens = 0
		s.rate = 0
		s.finished = false
	}
	if s.finished {
		return
	}
	chars := utf8.RuneCountInString(msg.Content().Text) + utf8.RuneCountInString(msg.ReasoningContent().Thinking)
	for _, call := range msg.ToolCalls() {
		chars += utf8.RuneCountInString(call.Input)
	}
	previous := s.tokens
	s.tokens = (chars + 3) / 4
	if s.tokens > 0 && s.started.IsZero() {
		s.started = now
	}
	if s.tokens != previous {
		s.samples = append(s.samples, throughputSample{at: now, tokens: s.tokens})
	}
	s.refresh(now)

	if !msg.IsFinished() {
		return
	}
	s.finished = true
	if s.tokens > 0 {
		duration := now.Sub(s.started)
		// A provider can deliver the whole response in one update. In that case,
		// measure the observed request duration instead of inventing stream timing.
		if len(s.samples) < 2 {
			duration = now.Sub(s.created)
		}
		s.rate = float64(s.tokens) / max(duration.Seconds(), 0.001)
	}
	if s.rate <= 0 || msg.FinishReason() == message.FinishReasonError || msg.FinishReason() == message.FinishReasonCanceled {
		return
	}
	s.total += s.rate
	s.count++
	s.history = append(s.history, s.rate)
	if len(s.history) > 12 {
		s.history = s.history[len(s.history)-12:]
	}
}

// refresh measures recent output over a two-second window. A stalled stream
// falls to zero; completed responses retain their final average.
func (s *throughputStats) refresh(now time.Time) {
	if s.finished || s.started.IsZero() {
		return
	}
	cutoff := now.Add(-2 * time.Second)
	for len(s.samples) > 1 && !s.samples[1].at.After(cutoff) {
		s.samples = s.samples[1:]
	}
	baseline := 0
	start := s.started
	if start.Before(cutoff) {
		start = cutoff
		if len(s.samples) > 0 && !s.samples[0].at.After(cutoff) {
			baseline = s.samples[0].tokens
		}
	}
	s.rate = float64(s.tokens-baseline) / max(now.Sub(start).Seconds(), 0.2)
}

func (s *throughputStats) sparkline() string {
	if len(s.history) == 0 {
		return ""
	}
	var peak float64
	for _, rate := range s.history {
		peak = max(peak, rate)
	}
	bars := []rune("▁▂▃▄▅▆▇█")
	var line strings.Builder
	for _, rate := range s.history {
		line.WriteRune(bars[min(7, int(rate/peak*7))])
	}
	return line.String()
}

func (m *editorCmp) throughputView() string {
	model := m.app.CoderAgent.Model()
	name := "No model"
	if fields := strings.Fields(model.Name); len(fields) > 0 {
		name = fields[0]
	}
	name = ansi.Truncate(name, max(4, m.width/3), "…")
	parts := []string{name, "— TPS"}
	if s := m.throughput[m.session.ID]; s != nil {
		if s.tokens > 0 {
			parts[1] = fmt.Sprintf("≈%.0f TPS", s.rate)
		}
		if graph := s.sparkline(); graph != "" {
			parts = append(parts, graph)
		}
		if s.count > 0 {
			parts = append(parts, fmt.Sprintf("avg ≈%.0f", s.total/float64(s.count)))
		}
		parts = append(parts, fmt.Sprintf("≈%d tokens", s.tokens))
	}
	// Drop lower priority fields before truncating model, speed, and history.
	for len(parts) > 3 && ansi.StringWidth(strings.Join(parts, " · ")) > m.width {
		parts = parts[:len(parts)-1]
	}
	t := theme.CurrentTheme()
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	for i, part := range parts {
		color := t.TextMuted()
		switch i {
		case 0:
			color = t.Text()
		case 1:
			color = t.Success()
		case 2:
			color = t.Secondary()
		}
		parts[i] = base.Foreground(color).Bold(i == 0).Render(part)
	}
	separator := base.Foreground(t.TextMuted()).Render(" · ")
	return styles.Surface(ansi.Truncate(strings.Join(parts, separator), max(1, m.width), "…"), t.BackgroundSecondary())
}
