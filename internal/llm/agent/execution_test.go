package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/provider"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
)

type executionMessages struct{ message.Service }

func (executionMessages) Create(_ context.Context, id string, p message.CreateMessageParams) (message.Message, error) {
	return message.Message{ID: "message", SessionID: id, Role: p.Role, Parts: p.Parts}, nil
}
func (executionMessages) Update(context.Context, message.Message) error { return nil }

type executionProvider struct{ provider.Provider }

func (executionProvider) Model() models.Model { return models.Model{ID: "fixture"} }
func (executionProvider) StreamResponse(context.Context, []message.Message, []tools.BaseTool) <-chan provider.ProviderEvent {
	events := make(chan provider.ProviderEvent, 1)
	events <- provider.ProviderEvent{Type: provider.EventToolUseStart, ToolCall: &message.ToolCall{ID: "command", Name: "failure"}}
	close(events)
	return events
}

type executionFailure struct{}

func (executionFailure) Info() tools.ToolInfo { return tools.ToolInfo{Name: "failure"} }
func (executionFailure) Run(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
	return tools.NewTextResponse("partial output"), errors.New("command timed out")
}

func TestExecutionErrorReachesModelAsFailedToolResult(t *testing.T) {
	a := &agent{messages: executionMessages{}, provider: executionProvider{}, tools: []tools.BaseTool{executionFailure{}}}
	_, result, err := a.streamAndHandleEvents(t.Context(), "session", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	results := result.ToolResults()
	require.Len(t, results, 1)
	require.True(t, results[0].IsError)
	require.Contains(t, results[0].Content, "command timed out")
	require.Contains(t, results[0].Content, "partial output")
}

type progressMessages struct {
	message.Service
	result    message.Message
	assistant message.Message
	creates   int
}

func (s *progressMessages) Create(_ context.Context, id string, p message.CreateMessageParams) (message.Message, error) {
	msg := message.Message{ID: string(p.Role), SessionID: id, Role: p.Role, Parts: p.Parts}
	if p.Role == message.Tool {
		s.result = msg
		s.creates++
	}
	return msg, nil
}
func (s *progressMessages) Update(_ context.Context, msg message.Message) error {
	if msg.Role == message.Tool {
		s.result = msg
	} else {
		s.assistant = msg
	}
	return nil
}

type progressProvider struct{ executionProvider }

func (progressProvider) StreamResponse(context.Context, []message.Message, []tools.BaseTool) <-chan provider.ProviderEvent {
	events := make(chan provider.ProviderEvent, 2) // Two fixed fixture events; no producer goroutine.
	for _, id := range []string{"first", "second"} {
		events <- provider.ProviderEvent{Type: provider.EventToolUseStart, ToolCall: &message.ToolCall{ID: id, Name: "progress", Finished: true}}
	}
	close(events)
	return events
}

type progressTool struct {
	t        *testing.T
	messages *progressMessages
}

func (progressTool) Info() tools.ToolInfo { return tools.ToolInfo{Name: "progress"} }
func (p progressTool) Run(_ context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	calls := p.messages.assistant.ToolCalls()
	if call.ID == "first" {
		require.Equal(p.t, "running", calls[0].Execution)
		require.Equal(p.t, "queued", calls[1].Execution)
	} else {
		results := p.messages.result.ToolResults()
		require.Len(p.t, results, 1, "first result must publish before second tool starts")
		require.Equal(p.t, "first", results[0].ToolCallID)
		require.Equal(p.t, "running", calls[1].Execution)
	}
	return tools.NewTextResponse(call.ID), nil
}
func TestToolResultsPublishBeforeNextToolStarts(t *testing.T) {
	storage := &progressMessages{}
	a := &agent{messages: storage, provider: progressProvider{}, tools: []tools.BaseTool{progressTool{t, storage}}}
	_, result, err := a.streamAndHandleEvents(t.Context(), "session", nil)
	require.NoError(t, err)
	require.Len(t, result.ToolResults(), 2)
	require.Equal(t, 1, storage.creates, "progress must update one result message")
}
