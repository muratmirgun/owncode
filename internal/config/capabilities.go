package config

import "github.com/muratmirgun/owncode/internal/llm/models"

// EffectiveCoder applies the active profile to the configured coding role.
func EffectiveCoder() Agent {
	if cfg == nil {
		return Agent{}
	}
	agent := cfg.Agents[AgentCoder]
	_, profile := CurrentProfile()
	if profile.Model != "" {
		agent.Model = profile.Model
		agent.MaxTokens = models.SupportedModels[profile.Model].DefaultMaxTokens
	}
	if profile.Reasoning != "" {
		agent.ReasoningEffort = profile.Reasoning
	}
	return agent
}

// CompactionUnavailable explains local capability failures before a request is sent.
// An empty result means the adapter is eligible, not that the remote service guarantees support.
func CompactionUnavailable(method string) string {
	if cfg == nil {
		return "Config is unavailable"
	}
	model := models.SupportedModels[EffectiveCoder().Model]
	switch method {
	case "shake":
		return ""
	case "summary":
		if model.ID == "" {
			return "Configure a model first"
		}
		return ""
	case "snapcompact":
		if !model.SupportsAttachments {
			return "Requires image input support"
		}
		return ""
	case "jev":
		if cfg.Compaction.Jev.APIKey == "" {
			return "Set compaction.jev.apiKey in the global config"
		}
		return ""
	case "native":
		switch model.Provider {
		case models.ProviderGemini, models.ProviderBedrock, models.ProviderAzure, models.ProviderVertexAI, models.ProviderCopilot:
			return "This adapter does not expose native compaction"
		}
		p := cfg.Providers[model.Provider]
		optedIn, _ := model.Options["native_compaction"].(bool)
		if model.ID != "" && (model.Provider == models.ProviderOpenAI || model.Provider == models.ProviderAnthropic || p.Auth == "chatgpt" || optedIn) {
			return ""
		}
		return "Requires OpenAI Responses, Claude compaction, or an opted-in compatible adapter"
	default:
		return "Unknown compaction method"
	}
}
