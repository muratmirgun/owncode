package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatGPTToCompatibleProviderUsesChatCompletions(t *testing.T) {
	native := message.NativeContext{Provider: "openai", Origin: "https://chatgpt.com/backend-api/codex", Model: "test", Data: json.RawMessage(`[{"type":"reasoning","encrypted_content":"secret-state"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Reading project"}]},{"type":"function_call","call_id":"call1","name":"view","arguments":"{}"}]`)}
	history := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{native, message.TextContent{Text: "Reading project"}, message.ToolCall{ID: "call1", Name: "view", Input: `{}`}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "call1", Content: "result"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "continue"}}},
	}
	before, err := json.Marshal(history)
	require.NoError(t, err)
	requests := make(chan map[string]json.RawMessage, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "wrong API", 400)
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", 400)
			return
		}
		requests <- body
		if string(body["stream"]) == "true" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
		}
	}))
	defer server.Close()
	p, err := NewProvider(models.ProviderOpenAI, WithAPIKey("test"), WithModel(models.Model{Provider: "theykk", APIModel: "qwen38", Custom: true}), WithOpenAIOptions(WithOpenAIBaseURL(server.URL)))
	require.NoError(t, err)
	_, err = p.SendMessages(context.Background(), history, nil)
	require.NoError(t, err)
	for event := range p.StreamResponse(context.Background(), history, nil) {
		require.NoError(t, event.Error)
	}
	for range 2 {
		body := <-requests
		var messages []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body["messages"], &messages))
		require.NotContains(t, string(body["messages"]), "secret-state")
		require.Contains(t, string(body["messages"]), "Reading project")
		require.Contains(t, string(body["messages"]), "call1")
	}
	after, err := json.Marshal(history)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
func TestPortableContextKeepsMatchingNativeAndRejectsCompaction(t *testing.T) {
	c := newOpenAIClient(providerClientOptions{model: models.Model{Provider: models.ProviderOpenAI, APIModel: "test"}}).(*openaiClient)
	native := message.NativeContext{Provider: "openai", Origin: c.nativeOrigin(), Model: "test", Data: json.RawMessage(`[{"type":"compaction","encrypted_content":"opaque"}]`)}
	history := []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{native}}}
	output, err := portableContext(history, c)
	require.NoError(t, err)
	require.Equal(t, history, output)
	_, err = portableContext(history, nil)
	require.ErrorContains(t, err, "switch back or start /new")
}
