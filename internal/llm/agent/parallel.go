package agent

import (
	"context"
	"sync"

	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
)

// runAgentBatch runs read-only children together and returns results in call order.
// The parent waits for every worker; cancellation also stops queued calls.
func runAgentBatch(ctx context.Context, tool tools.BaseTool, calls []message.ToolCall) []message.ToolResult {
	results := make([]message.ToolResult, len(calls))
	var workers sync.WaitGroup
	for worker := 0; worker < min(3, len(calls)); worker++ {
		workers.Go(func() {
			for i := worker; i < len(calls); i += 3 {
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
