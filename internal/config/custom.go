package config

import (
	"fmt"
	"net/url"

	"github.com/muratmirgun/owncode/internal/llm/models"
)

// CustomModel describes a model served by an OpenAI-compatible endpoint.
type CustomModel struct {
	Name          string         `json:"name"`
	ContextWindow int64          `json:"contextWindow"`
	MaxTokens     int64          `json:"maxTokens"`
	Reasoning     bool           `json:"reasoning"`
	Attachments   bool           `json:"attachments"`
	Interleaved   string         `json:"interleaved,omitempty"`
	Options       map[string]any `json:"options,omitempty"`
}

// Register only during startup, before agents read the model catalog.
func registerCustomModels(c *Config) error {
	pending := make(map[models.ModelID]models.Model)
	for providerID, provider := range c.Providers {
		if len(provider.Models) == 0 {
			continue
		}
		endpoint, err := url.Parse(provider.BaseURL)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return fmt.Errorf("provider %q requires an http or https baseURL", providerID)
		}
		for name, model := range provider.Models {
			if name == "" || model.Name == "" || model.ContextWindow <= 0 || model.MaxTokens <= 0 || model.MaxTokens > model.ContextWindow {
				return fmt.Errorf("provider %q model %q requires a name and valid contextWindow and maxTokens", providerID, name)
			}
			if model.Interleaved != "" && model.Interleaved != "reasoning_content" {
				return fmt.Errorf("provider %q model %q: interleaved must be reasoning_content", providerID, name)
			}
			id := models.ModelID(string(providerID) + "/" + name)
			if _, exists := models.SupportedModels[id]; exists {
				return fmt.Errorf("model %q is already registered", id)
			}
			pending[id] = models.Model{
				ID: id, Name: model.Name, Provider: providerID, APIModel: name,
				ContextWindow: model.ContextWindow, DefaultMaxTokens: model.MaxTokens,
				CanReason: model.Reasoning, SupportsAttachments: model.Attachments,
				Custom: true, ReasoningField: model.Interleaved, Options: model.Options,
			}
		}
	}
	for id, model := range pending {
		models.SupportedModels[id] = model
	}
	return nil
}
