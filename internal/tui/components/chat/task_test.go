package chat

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
)

type taskMessages struct {
	message.Service
	rows map[string][]message.Message
}

func (s taskMessages) List(_ context.Context, id string) ([]message.Message, error) {
	return s.rows[id], nil
}

type taskSessions struct{ session.Service }

func (taskSessions) Get(_ context.Context, id string) (session.Session, error) {
	return session.Session{ID: id, ParentSessionID: "s", Title: "Explore source"}, nil
}

func TestTaskCardStaysCompactAndOpensLiveConversation(t *testing.T) {
	m := scrollFixture()
	m.app.CoderAgent = workingAgent{}
	m.app.Sessions = taskSessions{}
	child := message.Message{ID: "child-msg", SessionID: "task", Role: message.Assistant, Model: models.BedrockClaude37Sonnet, Parts: []message.ContentPart{
		message.ToolCall{ID: "first", Name: "ls", Input: `{"path":"old"}`, Finished: true},
		message.ToolCall{ID: "last", Name: "ls", Input: `{"path":"latest"}`, Finished: true},
	}}
	m.app.Messages = taskMessages{rows: map[string][]message.Message{"task": {child}}}
	call := message.ToolCall{ID: "task", Name: agent.AgentToolName, Input: `{"prompt":"Explore source","role":"explore"}`, Finished: true}
	parent := message.Message{ID: "parent", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{call}}
	m.messages = append(m.messages, parent)
	m.renderView()
	if cmd := m.takeCommands(); cmd != nil {
		m.Update(cmd())
	}
	m.Update(conversationFrameMsg{owner: m})
	card := m.uiMessages[len(m.uiMessages)-1]
	require.Equal(t, 4, card.height)
	require.Contains(t, ansi.Strip(card.content), "Model: Bedrock: Claude 3.7 Sonnet · Effort: n/a")
	require.Contains(t, ansi.Strip(card.content), "latest")
	require.NotContains(t, ansi.Strip(card.content), "old")
	require.LessOrEqual(t, lipgloss.Width(card.content), m.width)
	live := message.Message{ID: "live-child", SessionID: "task", Role: message.Assistant, Model: models.BedrockClaude37Sonnet}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: live})
	m.Update(conversationFrameMsg{owner: m})
	require.Contains(t, ansi.Strip(m.uiMessages[len(m.uiMessages)-1].content), "Effort: n/a")
	live.Model = models.ModelID("live/custom")
	live.ReasoningEffort = "max"
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: live})
	m.Update(conversationFrameMsg{owner: m})
	card = m.uiMessages[len(m.uiMessages)-1]
	require.Contains(t, ansi.Strip(card.content), "Model: live/custom · Effort: max")
	m.viewport.GotoBottom()
	offset := m.viewport.YOffset()
	_, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 2, Y: card.position - offset})
	require.NotNil(t, cmd)
	m.Update(cmd())
	require.True(t, m.ReadOnly())
	require.Contains(t, ansi.Strip(m.View()), "Back to main chat")
	child.Parts = []message.ContentPart{message.TextContent{Text: "live update"}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: child})
	m.Update(conversationFrameMsg{owner: m.child})
	require.Contains(t, ansi.Strip(m.child.View()), "live update")
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.False(t, m.ReadOnly())
	require.Equal(t, offset, m.viewport.YOffset())
	// Reusing cached rows must preserve click coordinates.
	m.renderView()
	require.Equal(t, card.position, m.uiMessages[len(m.uiMessages)-1].position)
}

func TestChildViewKeepsOneAnimationClock(t *testing.T) {
	m := scrollFixture()
	m.app.CoderAgent = workingAgent{}
	_, cmd := m.Update(struct{}{})
	require.NotNil(t, cmd)
	require.True(t, m.animationPending)
	_, cmd = m.Update(struct{}{})
	require.Nil(t, cmd)
	m.child = NewMessagesCmp(m.app).(*messagesCmp)
	m.child.session = session.Session{ID: "child", ParentSessionID: "s"}
	_, cmd = m.Update(activityFrameMsg{owner: m})
	require.Nil(t, cmd)
	require.False(t, m.animationPending)
	require.False(t, m.child.animationPending)
	m.child = nil
	_, cmd = m.Update(struct{}{})
	require.NotNil(t, cmd)
	m.viewport, _ = m.viewport.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	_, cmd = m.Update(activityFrameMsg{owner: m})
	require.Nil(t, cmd)
	require.False(t, m.animationPending)
}

func TestStreamUpdatesCoalesceAndHiddenParentWaits(t *testing.T) {
	m := scrollFixture()
	msg := message.Message{ID: "a", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "before"}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	before := m.viewport.GetContent()
	for i := 0; i < 100; i++ {
		msg.Parts = []message.ContentPart{message.TextContent{Text: "latest output"}}
		_, cmd := m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
		if i == 0 {
			require.NotNil(t, cmd)
		} else {
			require.Nil(t, cmd)
		}
	}
	require.Equal(t, before, m.viewport.GetContent())
	m.Update(conversationFrameMsg{owner: m})
	require.Contains(t, ansi.Strip(m.viewport.GetContent()), "latest output")
	require.False(t, m.framePending)
	m.child = NewMessagesCmp(m.app).(*messagesCmp)
	m.child.session = session.Session{ID: "child", ParentSessionID: "s"}
	before = m.viewport.GetContent()
	msg.Parts = []message.ContentPart{message.TextContent{Text: "hidden update"}}
	_, cmd := m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
	require.Nil(t, cmd)
	require.Equal(t, before, m.viewport.GetContent())
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Contains(t, ansi.Strip(m.viewport.GetContent()), "hidden update")
}

func TestReadingChildHistoryDefersStreamWork(t *testing.T) {
	m := scrollFixture()
	m.session.ParentSessionID = "parent"
	msg := message.Message{ID: "live", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "before"}}}
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelUp})
	before := m.viewport.GetContent()
	offset := m.viewport.YOffset()
	for range 100 {
		msg.Parts = []message.ContentPart{message.TextContent{Text: "latest output"}}
		_, cmd := m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
		require.Nil(t, cmd)
	}
	// An already scheduled frame must also leave the reading surface untouched.
	m.Update(conversationFrameMsg{owner: m})
	require.Equal(t, before, m.viewport.GetContent())
	require.Equal(t, offset, m.viewport.YOffset())
	m.Update(tea.MouseWheelMsg{X: 2, Y: 2, Button: tea.MouseWheelUp})
	require.Less(t, m.viewport.YOffset(), offset)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	require.Contains(t, ansi.Strip(m.viewport.GetContent()), "latest output")
	require.True(t, m.viewport.AtBottom())
}

func TestLongReasoningHasCompactIndicator(t *testing.T) {
	msg := message.Message{ID: "thinking", Role: message.Assistant, Parts: []message.ContentPart{message.ReasoningContent{Thinking: strings.Repeat("reasoning content\n", 10000)}}}
	rows := renderAssistantMessage(msg, 0, nil, nil, "", false, 80, 0)
	require.Len(t, rows, 1)
	require.Equal(t, 1, rows[0].height)
	require.Contains(t, ansi.Strip(rows[0].content), "Thinking")
	require.Less(t, len(rows[0].content), 1000)
}

func TestMessagePanelsFitWidth(t *testing.T) {
	longID := models.ModelID("test/long-metadata")
	models.SupportedModels[longID] = models.Model{ID: longID, Name: strings.Repeat("Long model name ", 8), ReasoningLevels: []string{"extraordinarily-high"}}
	defer delete(models.SupportedModels, longID)
	children := []message.Message{{Role: message.Assistant, Model: longID, ReasoningEffort: "extraordinarily-high"}}
	for _, width := range []int{24, 60, 100} {
		view := renderMessage(strings.Repeat("A readable message. ", 8), true, false, width)
		require.Equal(t, width, lipgloss.Width(view))
		call := message.ToolCall{ID: "task", Name: agent.AgentToolName, Input: `{"prompt":"` + strings.Repeat("Long title ", 30) + `"}`, Finished: true}
		card := renderAgentCard(call, nil, children, width, 0)
		require.Equal(t, 4, card.height)
		require.Equal(t, width, lipgloss.Width(card.content))
		tool := renderToolMessage(message.ToolCall{ID: "ls", Name: "ls", Input: `{"path":"."}`, Finished: true}, nil, nil, "", false, width, 0)
		require.Equal(t, width, lipgloss.Width(tool.content))
	}
}

func TestAgentCardUsesLatestAssistantMetadata(t *testing.T) {
	customID := models.ModelID("custom/worker-model")
	models.SupportedModels[customID] = models.Model{ID: customID, Name: "Custom Worker", ReasoningLevels: []string{"low", "high"}}
	defer delete(models.SupportedModels, customID)

	call := message.ToolCall{ID: "task", Name: agent.AgentToolName, Input: `{"prompt":"Review","role":"witch-task-reviewer"}`}
	children := []message.Message{
		{Role: message.Assistant, Model: models.BedrockClaude37Sonnet},
		{Role: message.User, Model: models.ModelID("parent-must-not-win")},
		{Role: message.Assistant, Model: customID, ReasoningEffort: "high"},
	}
	card := renderAgentCard(call, nil, children, 100, 0)
	plain := ansi.Strip(card.content)
	require.Equal(t, 4, card.height)
	require.Contains(t, plain, "witch-task-reviewer")
	require.Contains(t, plain, "Model: Custom Worker · Effort: high")
	require.NotContains(t, plain, "Claude 3.7")
}

func TestAgentCardMetadataFallbacksAndStates(t *testing.T) {
	call := message.ToolCall{ID: "task", Name: agent.AgentToolName, Input: `{"prompt":"Explore"}`}
	queued := ansi.Strip(renderAgentCard(call, nil, nil, 60, 0).content)
	require.Contains(t, queued, "Model: — · Effort: —")
	require.Contains(t, queued, "Queued · click to open")

	unknown := message.Message{Role: message.Assistant, Model: models.ModelID("vendor/custom-id")}
	failedHistory := []message.Message{{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "task", IsError: true}}}}
	failed := ansi.Strip(renderAgentCard(call, failedHistory, []message.Message{unknown}, 60, 0).content)
	require.Contains(t, failed, "Model: vendor/custom-id · Effort: —")
	require.Contains(t, failed, "Failed · click to open")
}

func TestAgentCardLegacyReasoningModelWithoutSavedEffortIsUnavailable(t *testing.T) {
	call := message.ToolCall{ID: "task", Name: agent.AgentToolName, Input: `{"prompt":"Review"}`}
	child := message.Message{Role: message.Assistant, Model: models.Claude37Sonnet}
	card := ansi.Strip(renderAgentCard(call, nil, []message.Message{child}, 100, 0).content)
	require.Contains(t, card, "Model: Claude 3.7 Sonnet · Effort: —")
	require.NotContains(t, card, "Effort: n/a")
}
