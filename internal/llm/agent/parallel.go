package agent

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
)

// runAgentBatch runs children together and returns results in call order.
// The parent waits for every worker; cancellation also stops queued calls.
func runAgentBatch(ctx context.Context, tool tools.BaseTool, calls []message.ToolCall) []message.ToolResult {
	results := make([]message.ToolResult, len(calls))
	var workers sync.WaitGroup
	var next atomic.Int64
	for worker := 0; worker < min(3, len(calls)); worker++ {
		workers.Go(func() {
			// Claim the next call as soon as any worker finishes. Fixed stripes
			// leave idle workers waiting behind an unrelated long-running call.
			for {
				i := int(next.Add(1) - 1)
				if i >= len(calls) {
					return
				}
				call := calls[i]
				result := message.ToolResult{ToolCallID: call.ID}
				if ctx.Err() != nil {
					result.Content = "Tool execution canceled by user"
					result.IsError = true
				} else {
					response, err := tool.Run(ctx, tools.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input})
					result.Content, result.Metadata, result.IsError = response.Content, response.Metadata, response.IsError
					if err != nil {
						result.Content, result.IsError = err.Error(), true
					}
				}
				results[i] = result
			}
		})
	}
	workers.Wait()
	return results
}
