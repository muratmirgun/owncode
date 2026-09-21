package chat

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/history"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
)

type deferredMessages struct {
	message.Service
	calls int
}

func (s *deferredMessages) List(ctx context.Context, id string) ([]message.Message, error) {
	s.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []message.Message{{ID: "live", SessionID: id, Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "stored"}}}}, nil
}

func TestWorkerHistoryLoadsOutsideUpdateAndMergesLiveEvents(t *testing.T) {
	m := scrollFixture()
	storage := &deferredMessages{}
	m.app.Messages = storage
	m.app.Sessions = taskSessions{}
	cmd := m.openTask("worker")
	require.Zero(t, storage.calls, "opening a worker must not read storage in Update")
	require.True(t, m.ReadOnly())
	require.Contains(t, m.View(), "Loading")
	newer := message.Message{ID: "live", SessionID: "worker", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "new stream"}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: newer})
	m.Update(cmd())
	require.Equal(t, 1, storage.calls)
	require.Contains(t, ansi.Strip(m.child.View()), "new stream")
	require.NotContains(t, ansi.Strip(m.child.View()), "stored")
}

func TestStaleHistoryDoesNotReplaceNewSession(t *testing.T) {
	m := scrollFixture()
	storage := &deferredMessages{}
	m.app.Messages = storage
	old := m.SetSession(session.Session{ID: "old"})
	next := m.SetSession(session.Session{ID: "new"})
	require.Zero(t, storage.calls)
	m.Update(old())
	require.Equal(t, "new", m.session.ID)
	require.Empty(t, m.messages)
	m.Update(next())
	require.Equal(t, "new", m.messages[0].SessionID)
}

func TestLargeRenderKeepsInputAvailableAndRejectsOldSize(t *testing.T) {
	m := scrollFixture()
	text := strings.Repeat("A **long** paragraph with `code`.\n\n", 1000)
	msg := message.Message{ID: "large", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: text}}}
	_, cmd := m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	require.NotNil(t, cmd)
	require.True(t, m.textBusy)
	require.Contains(t, ansi.Strip(m.View()), "Rendering message")
	offset := m.viewport.YOffset()
	m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelUp})
	require.Less(t, m.viewport.YOffset(), offset, "input works before render completion")
	result := cmd()
	m.SetSize(80, 12)
	m.Update(result)
	require.Empty(t, m.textCache, "old width result must not enter the cache")
	// Returning to latest starts one render with the current width.
	_, next := m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	require.NotNil(t, next)
	m.Update(next())
	m.Update(conversationFrameMsg{owner: m})
	require.Contains(t, ansi.Strip(m.viewport.GetContent()), "long")
	require.Equal(t, 80, m.textCache["large"].width)
}

type sidebarHistory struct {
	history.Service
	calls int
}

func (s *sidebarHistory) ListLatestSessionFiles(context.Context, string) ([]history.File, error) {
	s.calls++
	return []history.File{{Path: "a.go", Version: "v1", Content: "new\n"}}, nil
}
func (s *sidebarHistory) ListBySession(context.Context, string) ([]history.File, error) {
	return []history.File{{ID: "initial", Path: "a.go", Version: history.InitialVersion, Content: "old\n"}}, nil
}
func TestSidebarUsesBackgroundRefreshAfterWorkerEvent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := config.Load(t.TempDir(), false)
	require.NoError(t, err)
	historyService := &sidebarHistory{}
	m := NewSidebarCmp(session.Session{}, historyService).(*sidebarCmp)
	require.Nil(t, m.Init(), "subscription belongs to the application")
	_, cmd := m.Update(SessionSelectedMsg{ID: "parent"})
	require.Zero(t, historyService.calls)
	_, ignored := m.Update(pubsub.Event[history.File]{Payload: history.File{SessionID: "worker"}})
	require.Nil(t, ignored)
	m.Update(cmd())
	require.Equal(t, 1, historyService.calls)
	_, next := m.Update(pubsub.Event[history.File]{Payload: history.File{SessionID: "parent"}})
	require.NotNil(t, next, "worker event must not stop later updates")
	m.Update(next())
	require.Equal(t, 2, historyService.calls)
	require.Contains(t, m.modFiles, "a.go")
}

func BenchmarkLargeStreamInputWhileRendering(b *testing.B) {
	m := scrollFixture()
	msg := message.Message{ID: "large", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("A worker reports **source findings**, `code` and next steps.\n\n", 1000)}}}
	_, job := m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
		m.Update(conversationFrameMsg{owner: m})
		m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelUp})
		_ = m.View()
	}
	b.StopTimer()
	if job != nil {
		m.Update(job())
	}
}

func TestBackgroundRenderDuringFiveWorkerStreams(t *testing.T) {
	m := scrollFixture()
	m.session.ParentSessionID = "parent"
	body := strings.Repeat("A **worker** inspects `source`.\n\n", 1000)
	msg := message.Message{ID: "live", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: body}}}
	_, job := m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	require.NotNil(t, job)
	done := make(chan tea.Msg, 1)
	go func() { done <- job() }()
	for range 100 {
		for _, id := range []string{"s", "worker2", "worker3", "worker4", "worker5"} {
			update := msg
			update.SessionID = id
			update.Parts = []message.ContentPart{message.TextContent{Text: body + "LATEST"}}
			m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: update})
		}
		m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelUp})
		_ = m.View()
	}
	require.True(t, m.ReadingHistory())
	m.Update(<-done)
	_, latest := m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	require.NotNil(t, latest)
	m.Update(latest())
	m.Update(conversationFrameMsg{owner: m})
	require.Contains(t, ansi.Strip(m.viewport.GetContent()), "LATEST")
	require.False(t, m.textBusy)
}
