package chat

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func scrollFixture() *messagesCmp {
	m := NewMessagesCmp(&app.App{}).(*messagesCmp)
	m.session = session.Session{ID: "s"}
	m.SetSize(60, 12)
	m.messages = []message.Message{{ID: "u", SessionID: "s", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("history line\n", 80)}}}}
	m.renderView()
	return m
}
func TestConversationMouseScrollAndBounds(t *testing.T) {
	m := scrollFixture()
	require.True(t, m.viewport.AtBottom())
	before := m.viewport.YOffset()
	m.Update(tea.MouseWheelMsg{X: 5, Y: 3, Button: tea.MouseWheelUp})
	require.Less(t, m.viewport.YOffset(), before)
	offset := m.viewport.YOffset()
	for _, point := range [][2]int{{65, 3}, {5, 15}} {
		m.Update(tea.MouseWheelMsg{X: point[0], Y: point[1], Button: tea.MouseWheelUp})
		require.Equal(t, offset, m.viewport.YOffset())
	}
	m.Update(tea.MouseWheelMsg{X: 5, Y: 3, Button: tea.MouseWheelDown})
	require.True(t, m.viewport.AtBottom())
}
func TestConversationKeepsReadingPositionDuringUpdates(t *testing.T) {
	m := scrollFixture()
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	offset := m.viewport.YOffset()
	require.False(t, m.viewport.AtBottom())
	newer := message.Message{ID: "new", SessionID: "s", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("new line\n", 15)}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: newer})
	require.Equal(t, offset, m.viewport.YOffset())
	newer.Parts = []message.ContentPart{message.TextContent{Text: strings.Repeat("updated line\n", 25)}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: newer})
	require.Equal(t, offset, m.viewport.YOffset())
	m.viewport.GotoBottom()
	newer.Parts = []message.ContentPart{message.TextContent{Text: strings.Repeat("updated line\n", 35)}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: newer})
	require.True(t, m.viewport.AtBottom())
}

type workingAgent struct{ agent.Service }

func (workingAgent) IsSessionBusy(string) bool { return true }
func TestActivityIndicatorShowsRequestPhase(t *testing.T) {
	m := NewMessagesCmp(&app.App{CoderAgent: workingAgent{}}).(*messagesCmp)
	m.width = 80
	require.Contains(t, ansi.Strip(m.working()), "Waiting for response")
	m.messages = []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "c", Name: "view", Finished: true}}}}
	require.Contains(t, ansi.Strip(m.working()), "Running tools")
	require.Contains(t, ansi.Strip(m.working()), "esc interrupt")
	require.Greater(t, len(m.spinner.Spinner.Frames), 1)
}

func BenchmarkConversationScroll(b *testing.B) {
	m := scrollFixture()
	m.app.CoderAgent = workingAgent{}
	m.messages[0].Parts = []message.ContentPart{message.TextContent{Text: strings.Repeat("long conversation with colored code and tool output\n", 10000)}}
	m.renderView()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		button := tea.MouseWheelUp
		if i%2 == 0 {
			button = tea.MouseWheelDown
		}
		m.Update(tea.MouseWheelMsg{X: 5, Y: 3, Button: button})
		_ = m.View()
	}
}

func BenchmarkConversationStreamUpdate(b *testing.B) {
	m := scrollFixture()
	m.messages[0].Parts = []message.ContentPart{message.TextContent{Text: strings.Repeat("long conversation with code and tools\n", 3000)}}
	m.renderView()
	msg := message.Message{ID: "stream", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("Some **markdown** output.\n", 50)}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
	}
}

func BenchmarkChildThinkingFrame(b *testing.B) {
	m := scrollFixture()
	m.session.ParentSessionID = "parent"
	msg := message.Message{ID: "thinking", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.ReasoningContent{Thinking: strings.Repeat("I will inspect the functions and compare their behavior.\n", 3000)}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
		m.Update(conversationFrameMsg{owner: m})
	}
}
