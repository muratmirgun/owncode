package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/require"
)

func TestCompatibleProviderStream(t *testing.T) {
	t.Parallel()
	requests := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request path or authentication")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		requests <- body
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`{"id":"test","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"Let me check."}}]}`,
			`{"id":"test","choices":[{"index":0,"delta":{"content":"Checking","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			`[DONE]`,
		}
		for _, chunk := range chunks {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", chunk); err != nil {
				t.Error(err)
				return
			}
		}
	}))
	defer server.Close()
	model := models.Model{APIModel: "qwen38", Custom: true, CanReason: true, ReasoningField: "reasoning_content", Options: map[string]any{
		"temperature": 0.6, "top_p": 0.95, "top_k": 20,
		"chat_template_kwargs": map[string]any{"enable_thinking": true, "preserve_thinking": true},
	}}
	client := newOpenAIClient(providerClientOptions{apiKey: "test-key", model: model, maxTokens: 32768, openaiOptions: []OpenAIOption{WithOpenAIBaseURL(server.URL + "/v1")}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var thinking string
	var response *ProviderResponse
	for event := range client.stream(ctx, []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}}}, nil) {
		require.NoError(t, event.Error)
		thinking += event.Thinking
		if event.Type == EventComplete {
			response = event.Response
		}
	}
	require.Equal(t, "Let me check.", thinking)
	require.NotNil(t, response)
	require.Equal(t, "Checking", response.Content)
	require.Equal(t, message.FinishReasonToolUse, response.FinishReason)
	require.Len(t, response.ToolCalls, 1)
	require.Equal(t, "lookup", response.ToolCalls[0].Name)
	body := <-requests
	require.Equal(t, "qwen38", body["model"])
	require.Equal(t, float64(32768), body["max_tokens"])
	require.Equal(t, float64(20), body["top_k"])
	require.Equal(t, 0.6, body["temperature"])
	require.Equal(t, 0.95, body["top_p"])
	require.Equal(t, true, body["chat_template_kwargs"].(map[string]any)["preserve_thinking"])
	require.NotContains(t, body, "tools")
	require.NotContains(t, body, "reasoning_effort")
	require.NotContains(t, body, "max_completion_tokens")
}

func TestCompatibleProviderPreservesReasoning(t *testing.T) {
	t.Parallel()
	client := &openaiClient{providerOptions: providerClientOptions{model: models.Model{ReasoningField: "reasoning_content"}}}
	messages := client.convertMessages([]message.Message{{Role: message.Assistant, Parts: []message.ContentPart{
		message.ReasoningContent{Thinking: "previous reasoning"},
		message.ToolCall{ID: "call_1", Name: "lookup", Input: "{}"},
	}}})
	data, err := json.Marshal(messages)
	require.NoError(t, err)
	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, "previous reasoning", decoded[1]["reasoning_content"])
	require.Len(t, decoded[1]["tool_calls"], 1)
}

func TestCompatibleProviderEmptyResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"choices":[]}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := newOpenAIClient(providerClientOptions{openaiOptions: []OpenAIOption{WithOpenAIBaseURL(server.URL)}})
	_, err := client.send(context.Background(), nil, nil)
	require.ErrorContains(t, err, "no completion choices")
}

func TestCompatibleProviderEmptyStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := newOpenAIClient(providerClientOptions{openaiOptions: []OpenAIOption{WithOpenAIBaseURL(server.URL)}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var streamError error
	for event := range client.stream(ctx, nil, nil) {
		streamError = event.Error
	}
	require.ErrorContains(t, streamError, "no completion choices")
}

func TestNativeOpenAIReasoningParams(t *testing.T) {
	t.Parallel()
	client := &openaiClient{providerOptions: providerClientOptions{model: models.Model{Provider: models.ProviderOpenAI, APIModel: "o3", CanReason: true}, maxTokens: 4096}}
	data, err := json.Marshal(client.preparedParams(nil, nil))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	require.Equal(t, float64(4096), body["max_completion_tokens"])
	require.Equal(t, "medium", body["reasoning_effort"])
	require.NotContains(t, body, "max_tokens")
}

func TestCompatibleProviderOmitsEmptyTools(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tools []tools.BaseTool
	}{
		{name: "nil"},
		{name: "empty", tools: []tools.BaseTool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if _, exists := body["tools"]; exists {
					http.Error(w, "tools must be omitted when empty", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Conversation summary"},"finish_reason":"stop"}]}`)
				if err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client := newOpenAIClient(providerClientOptions{
				model:         models.Model{APIModel: "qwen38", Custom: true},
				openaiOptions: []OpenAIOption{WithOpenAIBaseURL(server.URL)},
			})
			response, err := client.send(context.Background(), []message.Message{
				{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Summarize this conversation"}}},
			}, tc.tools)
			require.NoError(t, err)
			require.Equal(t, "Conversation summary", response.Content)
		})
	}
}

func TestOpenAIParamsPreserveTools(t *testing.T) {
	client := &openaiClient{}
	params := client.preparedParams(nil, []openai.ChatCompletionToolParam{
		{Function: openai.FunctionDefinitionParam{Name: "lookup"}},
	})
	data, err := json.Marshal(params)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	require.Len(t, body["tools"], 1)
}
