package chat

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
)

func TestThroughputHistory(t *testing.T) {
	s := &throughputStats{}
	now := time.Unix(100, 0)
	msg := message.Message{ID: "one", Role: message.Assistant}
	s.observe(msg, now)
	msg.AppendReasoningContent(strings.Repeat("x", 40))
	s.observe(msg, now.Add(time.Second))
	msg.AppendContent(strings.Repeat("x", 40))
	s.observe(msg, now.Add(2*time.Second))
	require.Equal(t, 20.0, s.rate)
	msg.AddFinish(message.FinishReasonEndTurn)
	s.observe(msg, now.Add(3*time.Second))
	s.observe(msg, now.Add(4*time.Second))
	require.Len(t, s.history, 1)
	require.Equal(t, 1, s.count)
	require.Equal(t, 10.0, s.history[0])
	require.NotEmpty(t, s.sparkline())
}

func TestThroughputIgnoresLoadedMessages(t *testing.T) {
	s := &throughputStats{}
	msg := message.Message{ID: "old"}
	msg.AppendContent("old text")
	msg.AddFinish(message.FinishReasonEndTurn)
	s.observe(msg, time.Now())
	require.Empty(t, s.history)
	require.Empty(t, s.messageID)
}

func TestThroughputSingleLine(t *testing.T) {
	m := &editorCmp{app: &app.App{CoderAgent: unconfiguredAgent{}}}
	for _, width := range []int{12, 20, 40, 80} {
		m.width = width
		view := m.throughputView()
		require.LessOrEqual(t, ansi.StringWidth(view), width)
		require.NotContains(t, view, "\n")
	}
}

func TestSlashCommandsWithoutModel(t *testing.T) {
	m := &editorCmp{textarea: textarea.New()}
	m.textarea.SetValue("/set")
	require.Equal(t, "settings", m.matchingCommands()[0].name)
	handled, _ := m.handleSlash(tea.KeyPressMsg{Code: tea.KeyTab})
	require.True(t, handled)
	require.Equal(t, "/settings", m.textarea.Value())
	handled, cmd := m.handleSlash(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, handled)
	require.NotNil(t, cmd)
	require.Empty(t, m.textarea.Value())
	m.textarea.SetValue("/")
	handled, _ = m.handleSlash(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.True(t, handled)
	require.Empty(t, m.matchingCommands())
	require.Equal(t, "/", m.textarea.Value())
}

func TestShortResponseHasTPS(t *testing.T) {
	s := &throughputStats{}
	now := time.Unix(100, 0)
	msg := message.Message{ID: "short", Role: message.Assistant}
	s.observe(msg, now)
	msg.AppendContent("hello there!")
	s.observe(msg, now.Add(10*time.Millisecond))
	msg.AddFinish(message.FinishReasonEndTurn)
	s.observe(msg, now.Add(12*time.Millisecond))
	require.Positive(t, s.rate)
	require.Len(t, s.history, 1)
	require.InDelta(t, 250, s.rate, 0.01)
}

func TestLiveTPSDropsWhenStreamStalls(t *testing.T) {
	s := &throughputStats{}
	now := time.Unix(100, 0)
	msg := message.Message{ID: "live", Role: message.Assistant}
	msg.AppendContent(strings.Repeat("x", 40))
	s.observe(msg, now)
	msg.AppendContent(strings.Repeat("x", 40))
	s.observe(msg, now.Add(time.Second))
	require.Positive(t, s.rate)
	s.refresh(now.Add(4 * time.Second))
	require.Zero(t, s.rate)
}

func TestThroughputTickerStopsAfterCompletion(t *testing.T) {
	m := &editorCmp{throughput: map[string]*throughputStats{"session": {messageID: "one", finished: true}}, throughputTicking: true}
	_, cmd := m.Update(throughputTickMsg(time.Now()))
	require.Nil(t, cmd)
	require.False(t, m.throughputTicking)
}

func TestThroughputUsesProviderCountAndStreamClock(t *testing.T) {
	s := &throughputStats{}
	start := time.Unix(100, 0)
	msg := message.Message{ID: "timed", Role: message.Assistant, StreamStartedAt: start, StreamUpdatedAt: start}
	msg.AppendContent("first batch")
	s.observe(msg, start.Add(3*time.Second))
	msg.AppendContent(" second batch")
	msg.StreamUpdatedAt = start.Add(time.Second)
	s.observe(msg, start.Add(6*time.Second))
	msg.StreamUpdatedAt = start.Add(2 * time.Second)
	msg.OutputTokens = 400
	msg.AddFinish(message.FinishReasonEndTurn)
	s.observe(msg, start.Add(10*time.Second))
	require.Equal(t, 200.0, s.rate)
	require.Equal(t, 400, s.tokens)
	require.True(t, s.exactTokens)
	require.Equal(t, []float64{200}, s.history)
}

func TestThroughputIncludesOnlyActiveConversationStreams(t *testing.T) {
	m := &editorCmp{app: &app.App{CoderAgent: unconfiguredAgent{}}, session: session.Session{ID: "parent"}, width: 100}
	for id, parent := range map[string]string{"child": "parent", "nested": "child", "finished": "parent", "unrelated": "other"} {
		m.Update(pubsub.Event[session.Session]{Type: pubsub.CreatedEvent, Payload: session.Session{ID: id, ParentSessionID: parent}})
	}
	m.throughput = map[string]*throughputStats{
		"parent":    {messageID: "p", rate: 20},
		"child":     {messageID: "c", rate: 30},
		"nested":    {messageID: "n", rate: 40},
		"finished":  {messageID: "f", rate: 900, finished: true},
		"unrelated": {messageID: "u", rate: 900},
	}
	rate, streams, workers := m.activeThroughput()
	require.Equal(t, 90.0, rate)
	require.Equal(t, 3, streams)
	require.Equal(t, 2, workers)
	require.Contains(t, ansi.Strip(m.throughputView()), "Σ ≈90 TPS (3 streams)")
	m.throughput["parent"].finished = true
	rate, streams, workers = m.activeThroughput()
	require.Equal(t, 70.0, rate)
	require.Equal(t, 2, streams)
	require.Equal(t, 2, workers)
	m.Update(pubsub.Event[session.Session]{Type: pubsub.DeletedEvent, Payload: session.Session{ID: "nested"}})
	rate, _, _ = m.activeThroughput()
	require.Equal(t, 30.0, rate)
}

func TestSlashModeArgumentsStayLocal(t *testing.T) {
	m := &editorCmp{textarea: textarea.New()}
	for _, value := range []string{"/fast on", "/fast off", "/yolo on", "/yolo off", "/fast invalid", "/yolo on extra"} {
		m.textarea.SetValue(value)
		handled, cmd := m.handleSlash(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.True(t, handled, value)
		require.NotNil(t, cmd)
		require.Empty(t, m.textarea.Value())
	}
}

func TestTTFTUsesRequestClockInsteadOfUIReceipt(t *testing.T) {
	start := time.Unix(100, 0)
	msg := message.Message{ID: "timed", Role: message.Assistant, RequestStartedAt: start, StreamStartedAt: start.Add(1500 * time.Millisecond), StreamUpdatedAt: start.Add(2 * time.Second), Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("x", 40)}}}
	s := &throughputStats{}
	s.observe(msg, start.Add(10*time.Second))
	require.Equal(t, 1500*time.Millisecond, s.started.Sub(s.created))
	require.Equal(t, 20.0, s.rate)
}

func TestPersistedDurationUsesSeconds(t *testing.T) {
	require.Equal(t, "3.0s", formatTimestampDiff(100, 103))
	require.Equal(t, "<1s", formatTimestampDiff(100, 100))
}
