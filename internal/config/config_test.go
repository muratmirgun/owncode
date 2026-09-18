package config

import (
	"testing"

	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/stretchr/testify/require"
)

func TestValidateWithoutAgents(t *testing.T) {
	original := cfg
	t.Cleanup(func() { cfg = original })
	cfg = &Config{}
	require.NoError(t, Validate())
}

func TestValidateConfiguredCoder(t *testing.T) {
	original := cfg
	t.Cleanup(func() { cfg = original })
	cfg = &Config{
		Agents: map[AgentName]Agent{
			AgentCoder: {Model: models.GPT41, MaxTokens: 4096},
		},
		Providers: map[models.ModelProvider]Provider{
			models.ProviderOpenAI: {APIKey: "test-only-not-a-real-key"},
		},
	}

	require.NoError(t, Validate())
	require.Equal(t, models.GPT41, cfg.Agents[AgentCoder].Model)
}
