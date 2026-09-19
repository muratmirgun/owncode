package provider

import (
	"encoding/json"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChatGPTResponseBodyUsesSelectedReasoning(t *testing.T) {
	m := models.Model{Provider: "chatgpt", APIModel: "test", ReasoningLevels: []string{"low", "high", "xhigh"}, DefaultReasoning: "high"}
	c := newOpenAIClient(providerClientOptions{model: m, openaiOptions: []OpenAIOption{WithChatGPT(), WithReasoningEffort("xhigh")}}).(*openaiClient)
	body, err := c.responseBody(nil, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"effort": "xhigh"}, body["reasoning"])
	m.ReasoningLevels = nil
	c.providerOptions.model = m
	body, err = c.responseBody(nil, nil)
	require.NoError(t, err)
	require.NotContains(t, body, "reasoning")
}

func TestCompatibleThinkingDoesNotMutateCatalogOptions(t *testing.T) {
	kwargs := map[string]any{"enable_thinking": true, "preserve_thinking": true}
	m := models.Model{Custom: true, CanReason: true, ReasoningField: "reasoning_content", Options: map[string]any{"chat_template_kwargs": kwargs}}
	c := newOpenAIClient(providerClientOptions{model: m, openaiOptions: []OpenAIOption{WithReasoningEffort("off")}}).(*openaiClient)
	data, err := json.Marshal(c.preparedParams(nil, nil))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	require.Equal(t, false, body["chat_template_kwargs"].(map[string]any)["enable_thinking"])
	require.Equal(t, true, kwargs["enable_thinking"])
	require.NotContains(t, body, "reasoning_effort")
}
