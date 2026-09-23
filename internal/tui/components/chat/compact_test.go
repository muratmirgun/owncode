package chat

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestToolCardExpandsAndKeepsErrorsVisible(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := config.Load(t.TempDir(), false)
	require.NoError(t, err)
	m := scrollFixture()
	call := message.Message{ID: "call", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "read", Name: "view", Input: `{"file_path":"main.go"}`, Finished: true}}}
	result := message.Message{ID: "result", SessionID: "s", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "read", Content: "package main\n// visible detail"}}}
	m.messages = []message.Message{call, result}
	m.renderView()
	require.Equal(t, 1, m.uiMessages[0].height)
	require.NotContains(t, ansi.Strip(m.View()), "visible detail")
	m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	require.True(t, m.expandedTools["read"])
	require.Contains(t, ansi.Strip(m.View()), "visible detail")
	m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	require.False(t, m.expandedTools["read"])
	require.Equal(t, 1, m.uiMessages[0].height)
	result.Parts = []message.ContentPart{message.ToolResult{ToolCallID: "read", Content: "permission denied", IsError: true}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: result})
	m.Update(conversationFrameMsg{owner: m})
	require.Contains(t, ansi.Strip(m.View()), "permission denied")
	require.Empty(t, m.uiMessages[0].toolID, "errors must remain open")
}

func TestClickNewMessagesReturnsToLive(t *testing.T) {
	m := scrollFixture()
	m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelUp})
	offset := m.viewport.YOffset()
	newer := message.Message{ID: "new", SessionID: "s", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "new incoming message"}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: newer})
	m.Update(conversationFrameMsg{owner: m})
	require.Equal(t, offset, m.viewport.YOffset())
	require.Contains(t, ansi.Strip(m.working()), "New messages")
	m.Update(tea.MouseClickMsg{X: 2, Y: m.viewport.Height(), Button: tea.MouseLeft})
	require.True(t, m.viewport.AtBottom())
	require.False(t, m.dirty)
	require.Contains(t, ansi.Strip(m.View()), "new incoming message")
}

func TestToolBurstUsesOneFrameAndLatestResult(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := config.Load(t.TempDir(), false)
	require.NoError(t, err)
	m := scrollFixture()
	call := message.Message{ID: "call", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "read", Name: "view", Input: `{"file_path":"main.go"}`, Finished: true}}}
	m.messages = append(m.messages, call)
	m.renderView()
	revision := m.revision
	result := message.Message{ID: "result", SessionID: "s", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "read", Content: "first"}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: result})
	for range 100 {
		result.Parts = []message.ContentPart{message.ToolResult{ToolCallID: "read", Content: "latest"}}
		m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: result})
	}
	require.Equal(t, revision, m.revision, "updates must not force individual renders")
	require.True(t, m.framePending)
	m.Update(conversationFrameMsg{owner: m})
	require.Equal(t, revision+1, m.revision)
	require.False(t, m.framePending)
	require.Equal(t, "latest", findToolResponse("read", m.messages).Content)
}

func TestToolPreviewBoundsLongLines(t *testing.T) {
	preview := truncateHeight(strings.Repeat("世", 100000), 200)
	require.True(t, utf8.ValidString(preview))
	require.Less(t, len(preview), 33000)
	require.Contains(t, preview, "additional output omitted")
}

func TestWorkerIndexTracksCallsAddedDuringStreaming(t *testing.T) {
	m := scrollFixture()
	parent := message.Message{ID: "parent", SessionID: "s", Role: message.Assistant}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: parent})
	parent.Parts = []message.ContentPart{message.ToolCall{ID: "worker", Name: "agent", Input: `{"prompt":"Inspect"}`, Finished: true}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: parent})
	child := message.Message{ID: "child", SessionID: "worker", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "latest worker activity"}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: child})
	require.Equal(t, "latest worker activity", m.taskHistory["worker"][0].Content().Text)
	require.Equal(t, []string{"parent"}, m.taskParents["worker"])
	m.Update(SessionClearedMsg{})
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: child})
	require.Empty(t, m.taskParents)
}
