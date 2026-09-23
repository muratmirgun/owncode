package provider

import (
	"encoding/json"
	"testing"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/stretchr/testify/require"
)

func TestFastTierChangesExistingClientsWithoutChangingReasoning(t *testing.T) {
	previous := config.FastMode()
	t.Cleanup(func() { config.SetFastMode(previous) })
	for _, provider := range []models.ModelProvider{models.ProviderOpenAI, "chatgpt", "theykk"} {
		t.Run(string(provider), func(t *testing.T) {
			m := models.Model{Provider: provider, APIModel: "test", CanReason: true, ReasoningLevels: []string{"low", "high"}}
			c := newOpenAIClient(providerClientOptions{model: m, openaiOptions: []OpenAIOption{WithReasoningEffort("high")}}).(*openaiClient)
			if provider == "chatgpt" {
				c.options.chatGPT = true
			}
			for _, enabled := range []bool{false, true, false} {
				config.SetFastMode(enabled)
				params := c.preparedParams(nil, nil)
				data, err := json.Marshal(params)
				require.NoError(t, err)
				var body map[string]any
				require.NoError(t, json.Unmarshal(data, &body))
				require.Equal(t, "high", body["reasoning_effort"])
				if provider == "theykk" {
					require.NotContains(t, body, "service_tier")
					continue
				}
				expected := "default"
				if enabled {
					expected = "priority"
				}
				require.Equal(t, expected, body["service_tier"])
				native, err := c.responseBody(nil, nil)
				require.NoError(t, err)
				require.Equal(t, expected, native["service_tier"])
				require.Equal(t, map[string]any{"effort": "high"}, native["reasoning"])
			}
		})
	}
}
