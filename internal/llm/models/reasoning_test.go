package models

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReasoningUsesAdvertisedLevelsAndDefaults(t *testing.T) {
	m := Model{Provider: "chatgpt", ReasoningLevels: []string{"low", "high", "xhigh"}, DefaultReasoning: "high"}
	require.Equal(t, "high", m.ReasoningLevel("medium"))
	require.Equal(t, "xhigh", m.ReasoningLevel("xhigh"))
	choices := m.ReasoningChoices()
	choices[0] = "changed"
	require.Equal(t, "low", m.ReasoningLevels[0])
	require.Empty(t, (Model{}).ReasoningChoices())
}

func TestThinkingToggleUsesModelDefault(t *testing.T) {
	m := Model{CanReason: true, Custom: true, ReasoningField: "reasoning_content", Options: map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": false}}}
	require.Equal(t, []string{"off", "on"}, m.ReasoningChoices())
	require.Equal(t, "off", m.ReasoningLevel(""))
	require.Equal(t, "on", m.ReasoningLevel("on"))
	m = Model{Provider: ProviderAnthropic, CanReason: true}
	require.Equal(t, "auto", m.ReasoningLevel(""))
}
