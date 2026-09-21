package agent

import (
	"context"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestJevSummaryFallbackPolicy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, method                                string
		failed, enabled, available, cancelled, want bool
	}{
		{"enabled", "jev", true, true, true, false, true},
		{"disabled", "jev", true, false, true, false, false},
		{"successful", "jev", false, true, true, false, false},
		{"unconfigured", "jev", true, true, false, false, false},
		{"cancelled", "jev", true, true, true, true, false},
		{"native", "native", true, true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			got := shouldFallbackJev(ctx, tc.method, compactResult{fallback: tc.failed}, config.JevSettings{SummaryFallback: tc.enabled}, tc.available)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestJevProtectsWritableAndResumedWorkers(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`{"role":"implement"}`, `{"worker_id":"existing"}`, `{"role":"witch-builder"}`} {
		msgs := []message.Message{
			{Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "worker", Name: AgentToolName, Input: input, Finished: true}, message.ToolCall{ID: "read", Name: "view", Input: `{}`, Finished: true}}},
			{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "worker", Content: strings.Repeat("important edit record ", 100)}, message.ToolResult{ToolCallID: "read", Content: strings.Repeat("obsolete read output ", 3000)}}},
		}
		for range 4 {
			msgs = append(msgs, message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "continue"}}})
		}
		result, err := compactWithJev(t.Context(), msgs, config.JevSettings{}, t.TempDir(), "continue", lowLossScorer{})
		require.NoError(t, err)
		require.Equal(t, msgs[0], result.messages[0])
		require.Equal(t, msgs[1].ToolResults()[0], result.messages[1].ToolResults()[0])
		require.Less(t, len(result.messages[2].ToolResults()[0].Content), len(msgs[1].ToolResults()[1].Content))
	}
}
