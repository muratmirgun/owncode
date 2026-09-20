package agent

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
)

type batchTool struct {
	tools.BaseTool
	run func(context.Context, tools.ToolCall) (tools.ToolResponse, error)
}

func (b batchTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	return b.run(ctx, call)
}

func TestAgentBatchOverlapsAndKeepsOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started := make(chan string)
	release := make(chan struct{})
	var active atomic.Int32
	var peak atomic.Int32
	tool := batchTool{run: func(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		select {
		case started <- call.ID:
		case <-ctx.Done():
			return tools.ToolResponse{}, ctx.Err()
		}
		select {
		case <-release:
		case <-ctx.Done():
			return tools.ToolResponse{}, ctx.Err()
		}
		return tools.NewTextResponse(call.ID), nil
	}}
	calls := make([]message.ToolCall, 6)
	for i := range calls {
		calls[i] = message.ToolCall{ID: fmt.Sprint(i), Name: AgentToolName}
	}
	done := make(chan []message.ToolResult, 1)
	go func() { done <- runAgentBatch(ctx, tool, calls) }()
	// All three must start before any can finish.
	for range 3 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("agents did not overlap")
		}
	}
	close(release)
	for range 3 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("queued agents did not start")
		}
	}
	results := <-done
	require.Equal(t, int32(3), peak.Load())
	for i, result := range results {
		require.Equal(t, calls[i].ID, result.ToolCallID)
		require.Equal(t, calls[i].ID, result.Content)
	}
}

func TestAgentBatchCancellationAndErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tool := batchTool{run: func(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
		return tools.ToolResponse{}, errors.New("child failed")
	}}
	calls := []message.ToolCall{{ID: "one"}, {ID: "two"}}
	for _, result := range runAgentBatch(ctx, tool, calls) {
		require.True(t, result.IsError)
		require.Contains(t, result.Content, "canceled")
	}
	for _, result := range runAgentBatch(context.Background(), tool, calls) {
		require.True(t, result.IsError)
		require.Equal(t, "child failed", result.Content)
	}
}
