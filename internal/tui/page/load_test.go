package page

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/components/chat"
	"github.com/stretchr/testify/require"
)

type loadAgent struct{ unconfiguredAgent }

func (loadAgent) IsSessionBusy(string) bool { return true }

type loadMessages struct{ message.Service }

func (loadMessages) List(_ context.Context, id string) ([]message.Message, error) {
	return []message.Message{{ID: id + "-user", SessionID: id, Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("Inspect the source code and report findings.\n\n", 60)}}}, {ID: id + "-live", SessionID: id, Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Initial output"}}}}, nil
}

type loadSessions struct{ session.Service }

func (loadSessions) Get(_ context.Context, id string) (session.Session, error) {
	return session.Session{ID: id, ParentSessionID: "parent", Title: "Inspect source"}, nil
}

func fourAgentPage(b testing.TB) *chatPage {
	b.Setenv("HOME", b.TempDir())
	_, err := config.Load(b.TempDir(), false)
	require.NoError(b, err)
	p := NewChatPage(&app.App{CoderAgent: loadAgent{}, Messages: loadMessages{}, Sessions: loadSessions{}}).(*chatPage)
	p.session = session.Session{ID: "parent"}
	p.SetSize(220, 70)
	_, cmd := p.messages.Update(chat.SessionSelectedMsg(p.session))
	if cmd != nil {
		p.messages.Update(cmd())
	}
	for i := 0; i < 4; i++ {
		call := message.ToolCall{ID: fmt.Sprint("task", i), Name: "agent", Input: `{"prompt":"Inspect source"}`, Finished: true}
		p.messages.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: message.Message{ID: call.ID + "-parent", SessionID: "parent", Role: message.Assistant, Parts: []message.ContentPart{call}}})
	}
	// Find a visible card without depending on its rendered row count.
	for y := 1; y < 65; y++ {
		_, cmd := p.messages.Update(tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseLeft})
		if cmd != nil {
			p.messages.Update(cmd())
		}
		if p.messages.(interface{ ReadOnly() bool }).ReadOnly() {
			break
		}
	}
	require.True(b, p.messages.(interface{ ReadOnly() bool }).ReadOnly())
	p.Update(tea.MouseWheelMsg{X: 5, Y: 8, Button: tea.MouseWheelUp})
	require.True(b, p.messages.(interface{ ReadingHistory() bool }).ReadingHistory())
	return p
}

func BenchmarkFourAgentPage(b *testing.B) {
	p := fourAgentPage(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprint("task", i%4)
		p.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: message.Message{ID: id + "-live", SessionID: id, Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Streaming output"}}}})
		_ = p.View()
	}
}

func BenchmarkFourAgentScroll(b *testing.B) {
	p := fourAgentPage(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprint("task", i%4)
		p.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: message.Message{ID: id + "-live", SessionID: id, Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprint("Output ", i)}}}})
		button := tea.MouseWheelUp
		if i%2 == 0 {
			button = tea.MouseWheelDown
		}
		p.Update(tea.MouseWheelMsg{X: 5, Y: 8, Button: button})
		_ = p.View()
	}
}

func TestHistoryFrameCacheInvalidation(t *testing.T) {
	p := fourAgentPage(t)
	_ = p.View()
	require.Len(t, p.scrollFrames, 1)
	p.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: message.Message{ID: "task0-live", SessionID: "task0", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "new output"}}}})
	_ = p.View()
	require.NotEmpty(t, p.scrollFrames)
	p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Empty(t, p.scrollFrames, "returning to the parent must discard child frames")
	_ = p.View()
	p.SetSize(180, 60)
	require.Empty(t, p.scrollFrames, "resize must discard frames")
}
