package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/require"
)

func TestChatGPTUsesResponsesForChatAndSummary(t *testing.T) {
	requests := make(chan map[string]json.RawMessage, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.Error(w, "wrong endpoint", 400)
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", 400)
			return
		}
		requests <- body
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}],\"usage\":{\"input_tokens\":100,\"output_tokens\":1}}}\n\n")
	}))
	defer server.Close()
	c := newOpenAIClient(providerClientOptions{model: models.Model{Provider: "chatgpt", APIModel: "test"}, maxTokens: 8000, openaiOptions: []OpenAIOption{WithChatGPT()}}).(*openaiClient)
	// Replace the HTTP client only; retain the production ChatGPT routing flag.
	c.client = openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"))
	history := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}}}
	var complete *ProviderResponse
	for event := range c.stream(context.Background(), history, nil) {
		require.NoError(t, event.Error)
		if event.Type == EventComplete {
			complete = event.Response
		}
	}
	require.NotNil(t, complete)
	require.Equal(t, "hello", complete.Content)
	require.NotNil(t, complete.Native)
	result, err := c.send(context.Background(), history, nil)
	require.NoError(t, err)
	require.Equal(t, "hello", result.Content)
	for range 2 {
		body := <-requests
		require.Equal(t, "true", string(body["stream"]))
		require.Equal(t, "false", string(body["store"]))
		require.NotContains(t, body, "max_output_tokens")
		require.NotContains(t, body, "tools")
		require.Contains(t, string(body["input"]), "hello")
	}
}

func TestChatGPTPreservesStreamedItemsWithEmptyFinalOutput(t *testing.T) {
	for _, terminalOutput := range []string{`[]`, `null`, ``} {
		t.Run("output="+terminalOutput, func(t *testing.T) {
			reasoning := json.RawMessage(`{"type":"reasoning","id":"r1","encrypted_content":"opaque-state","summary":[]}`)
			call := json.RawMessage(`{"type":"function_call","id":"f1","call_id":"call1","name":"agent","arguments":"{\"prompt\":\"Read the project\"}"}`)
			text := json.RawMessage(`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I will inspect it."}]}`)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				// Indices, not delivery order, determine replay order.
				fmt.Fprintf(w, "data: {\"type\":\"response.output_item.done\",\"output_index\":1,\"item\":%s}\n\n", call)
				fmt.Fprintf(w, "data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":%s}\n\n", reasoning)
				fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"I will \"}\n\n")
				fmt.Fprintf(w, "data: {\"type\":\"response.output_item.done\",\"output_index\":2,\"item\":%s}\n\n", text)
				output := ""
				if terminalOutput != "" {
					output = `,"output":` + terminalOutput
				}
				fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"%s}}\n\n", output)
			}))
			defer server.Close()
			c := newOpenAIClient(providerClientOptions{model: models.Model{Provider: "chatgpt", APIModel: "test"}, openaiOptions: []OpenAIOption{WithChatGPT()}}).(*openaiClient)
			c.client = openai.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"))
			var complete *ProviderResponse
			var content string
			for event := range c.stream(context.Background(), nil, nil) {
				require.NoError(t, event.Error)
				if event.Type == EventContentDelta {
					content += event.Content
				}
				if event.Type == EventComplete {
					complete = event.Response
				}
			}
			require.NotNil(t, complete)
			require.Equal(t, "I will inspect it.", content)
			require.Equal(t, message.FinishReasonToolUse, complete.FinishReason)
			require.Len(t, complete.ToolCalls, 1)
			require.Equal(t, "agent", complete.ToolCalls[0].Name)
			require.Equal(t, "call1", complete.ToolCalls[0].ID)
			require.JSONEq(t, `{"prompt":"Read the project"}`, complete.ToolCalls[0].Input)
			want, _ := json.Marshal([]json.RawMessage{reasoning, call, text})
			require.JSONEq(t, string(want), string(complete.Native.Data))
			replay, err := c.responseInput([]message.Message{
				{Role: message.Assistant, Parts: []message.ContentPart{*complete.Native}},
				{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "call1", Content: "Project inspected"}}},
			})
			require.NoError(t, err)
			encoded, err := json.Marshal(replay)
			require.NoError(t, err)
			require.Contains(t, string(encoded), "function_call_output")
			require.Contains(t, string(encoded), "opaque-state")
		})
	}
}
