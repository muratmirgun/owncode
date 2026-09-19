package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/message"
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
	handled, _ := m.handleSlash(tea.KeyMsg{Type: tea.KeyTab})
	require.True(t, handled)
	require.Equal(t, "/settings", m.textarea.Value())
	handled, cmd := m.handleSlash(tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, handled)
	require.NotNil(t, cmd)
	require.Empty(t, m.textarea.Value())
	m.textarea.SetValue("/")
	handled, _ = m.handleSlash(tea.KeyMsg{Type: tea.KeyEsc})
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
