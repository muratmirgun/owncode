package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
)

func TestOpenAINativeCompactAndStreamingReplay(t *testing.T) {
	opaque := `[{"type":"message","role":"user","content":[{"type":"input_text","text":"keep"}]},{"type":"compaction","id":"cmp_1","encrypted_content":"opaque-data"}]`
	output := `[{"type":"reasoning","id":"r1","encrypted_content":"reasoning-state","summary":[]},{"type":"function_call","id":"f1","call_id":"call1","name":"view","arguments":"{}"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		var body map[string]json.RawMessage
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/responses/compact" {
			require.NotContains(t, body, "tools")
			fmt.Fprintf(w, `{"output":%s,"usage":{"input_tokens":1000,"output_tokens":100}}`, opaque)
			return
		}
		require.Equal(t, "/responses", r.URL.Path)
		var input []json.RawMessage
		require.NoError(t, json.Unmarshal(body["input"], &input))
		require.Len(t, input, 3)
		require.JSONEq(t, `{"type":"compaction","id":"cmp_1","encrypted_content":"opaque-data"}`, string(input[1]))
		require.Equal(t, "false", string(body["store"]))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":%s,\"usage\":{\"input_tokens\":110,\"output_tokens\":10}}}\n\n", output)
	}))
	defer server.Close()
	p, err := NewProvider(models.ProviderOpenAI, WithAPIKey("test-key"), WithModel(models.Model{Provider: models.ProviderOpenAI, APIModel: "test", SupportsAttachments: true}), WithMaxTokens(1000), WithOpenAIOptions(WithOpenAIBaseURL(server.URL)))
	require.NoError(t, err)
	compacted, err := p.(NativeCompactor).Compact(context.Background(), []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}}}, nil, "focus")
	require.NoError(t, err)
	require.JSONEq(t, opaque, string(compacted.Context.Data))
	require.Equal(t, int64(100), compacted.Usage.OutputTokens)
	history := []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{compacted.Context}}, {Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "next"}}}}
	var complete *ProviderResponse
	text := ""
	for event := range p.StreamResponse(context.Background(), history, nil) {
		require.NoError(t, event.Error)
		if event.Type == EventContentDelta {
			text += event.Content
		}
		if event.Type == EventComplete {
			complete = event.Response
		}
	}
	require.Equal(t, "hello", text)
	require.NotNil(t, complete)
	require.Len(t, complete.ToolCalls, 1)
	require.Equal(t, "call1", complete.ToolCalls[0].ID)
	require.JSONEq(t, output, string(complete.Native.Data))
	client := newOpenAIClient(providerClientOptions{model: models.Model{Provider: models.ProviderOpenAI, APIModel: "other"}}).(*openaiClient)
	_, err = client.responseInput(history)
	require.ErrorContains(t, err, "switch back")
}

func TestClaudeNativeCompactAndReplay(t *testing.T) {
	block := `[{"type":"compaction","content":"summary","signature":"signed"}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		require.Equal(t, compactBeta, r.Header.Get("anthropic-beta"))
		var body map[string]json.RawMessage
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if _, ok := body["compaction"]; ok {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"content":%s,"stop_reason":"compaction","usage":{"iterations":[{"input_tokens":1000,"output_tokens":100}]}}`, block)
			return
		}
		var msgs []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body["messages"], &msgs))
		require.JSONEq(t, block, string(msgs[0]["content"]))
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":110}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}`,
			`{"type":"message_stop"}`,
		} {
			fmt.Fprintf(w, "data: %s\n\n", event)
		}
	}))
	defer server.Close()
	client := &anthropicClient{providerOptions: providerClientOptions{model: models.Model{APIModel: "claude-opus-5"}, maxTokens: 4096}, client: anthropic.NewClient(anthropicoption.WithAPIKey("test"), anthropicoption.WithBaseURL(server.URL))}
	compacted, err := client.nativeCompact(context.Background(), []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}}}, nil, "focus")
	require.NoError(t, err)
	require.Equal(t, int64(1000), compacted.Usage.InputTokens)
	history := []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{compacted.Context}}, {Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "continue"}}}}
	var complete *ProviderResponse
	for event := range client.nativeStream(context.Background(), history, nil) {
		require.NoError(t, event.Error)
		if event.Type == EventComplete {
			complete = event.Response
		}
	}
	require.NotNil(t, complete)
	require.Equal(t, "hello", complete.Content)
	require.Equal(t, int64(110), complete.Usage.InputTokens)
	require.JSONEq(t, `[{"type":"text","text":"hello"}]`, string(complete.Native.Data))
}

func TestNativeRejectsUnsupportedProviderAndInvalidOutput(t *testing.T) {
	client := newOpenAIClient(providerClientOptions{model: models.Model{Provider: "theykk", APIModel: "qwen38"}}).(*openaiClient)
	_, err := client.nativeCompact(context.Background(), nil, nil, "")
	require.ErrorContains(t, err, "Responses API")
	require.False(t, containsNativeBlock([]byte(`[{"type":"compaction","content":""}]`), "compaction", "content"))
	require.Error(t, validateContext([]message.Message{{Parts: []message.ContentPart{message.ContextSnapshot{}}}}, models.Model{}))
	require.Error(t, validateContext([]message.Message{{Parts: []message.ContentPart{message.BinaryContent{}}}}, models.Model{}))
}

type schemaTool struct{ tools.BaseTool }

func (schemaTool) Info() tools.ToolInfo { return tools.ToolInfo{Name: "example"} }

func TestNativeToolsEmitValidEmptySchema(t *testing.T) {
	data, err := json.Marshal(nativeTools([]tools.BaseTool{schemaTool{}}, false))
	require.NoError(t, err)
	require.Contains(t, string(data), `"required":[]`)
	require.Contains(t, string(data), `"properties":{}`)
}

func TestNativeStreamCancellationClosesChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := nativeEvents(ctx, func(emit func(ProviderEvent) bool) error {
		for emit(ProviderEvent{Type: EventContentDelta, Content: "text"}) {
		}
		return ctx.Err()
	})
	<-events
	cancel()
	for range events {
	}
}
