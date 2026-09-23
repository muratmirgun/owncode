package util

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/message"
)

// WorkerMetadata returns recorded worker settings without guessing from current configuration.
func WorkerMetadata(msg *message.Message) (name, effort string) {
	name, effort = "—", "—"
	if msg == nil || msg.Model == "" {
		return name, effort
	}
	name = string(msg.Model)
	if model, ok := models.SupportedModels[msg.Model]; ok {
		if model.Name != "" {
			name = model.Name
		}
		if len(model.ReasoningChoices()) == 0 {
			effort = "n/a"
		}
	}
	if msg.ReasoningEffort != "" {
		effort = msg.ReasoningEffort
	}
	return strings.Join(strings.Fields(name), " "), strings.Join(strings.Fields(effort), " ")
}

// WorkerMetadataLine reserves space for reasoning before shortening the model name.
func WorkerMetadataLine(msg *message.Message, width int) string {
	if width <= 0 {
		return ""
	}
	name, effort := WorkerMetadata(msg)
	suffix := " · Effort: " + effort
	if width >= ansi.StringWidth(suffix)+8 {
		return "Model: " + ansi.Truncate(name, width-7-ansi.StringWidth(suffix), "…") + suffix
	}
	// Compact labels on narrow terminals, while retaining both values when possible.
	if width < 5 {
		return ansi.Truncate(effort, width, "…")
	}
	effort = ansi.Truncate(effort, min(ansi.StringWidth(effort), width-4), "…")
	return ansi.Truncate(name, width-3-ansi.StringWidth(effort), "…") + " · " + effort
}
