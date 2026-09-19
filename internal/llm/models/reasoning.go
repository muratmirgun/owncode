package models

import "slices"

// ReasoningChoices returns only levels declared by a model or its supported adapter.
func (m Model) ReasoningChoices() []string {
	if len(m.ReasoningLevels) > 0 {
		return slices.Clone(m.ReasoningLevels)
	}
	if !m.CanReason {
		return nil
	}
	if m.Provider == ProviderAnthropic {
		return []string{"auto", "off", "on"}
	}
	if !m.Custom && (m.Provider == ProviderOpenAI || m.Provider == ProviderAzure || m.Provider == ProviderCopilot || m.Provider == ProviderLocal) {
		return []string{"low", "medium", "high"}
	}
	if m.ReasoningField == "reasoning_content" {
		if options, ok := m.Options["chat_template_kwargs"].(map[string]any); ok {
			if _, ok := options["enable_thinking"].(bool); ok {
				return []string{"off", "on"}
			}
		}
	}
	return nil
}

// ReasoningLevel normalizes a saved selection against the model's available choices.
func (m Model) ReasoningLevel(saved string) string {
	choices := m.ReasoningChoices()
	if slices.Contains(choices, saved) {
		return saved
	}
	if slices.Contains(choices, m.DefaultReasoning) {
		return m.DefaultReasoning
	}
	if slices.Contains(choices, "medium") {
		return "medium"
	}
	if slices.Contains(choices, "auto") {
		return "auto"
	}
	if slices.Contains(choices, "on") {
		if options, ok := m.Options["chat_template_kwargs"].(map[string]any); ok && options["enable_thinking"] == false {
			return "off"
		}
		return "on"
	}
	if len(choices) > 0 {
		return choices[0]
	}
	return ""
}
