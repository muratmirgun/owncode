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
