package util

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
)

func TestWorkerMetadataKeepsReasoningVisible(t *testing.T) {
	msg := &message.Message{Model: models.ModelID(strings.Repeat("very-long-worker-model-", 8)), ReasoningEffort: "high"}
	for _, width := range []int{12, 21, 40, 80} {
		line := WorkerMetadataLine(msg, width)
		require.LessOrEqual(t, ansi.StringWidth(line), width)
		require.Contains(t, line, "high")
		require.Contains(t, line, "…")
	}
	for width := 0; width < 12; width++ {
		require.LessOrEqual(t, ansi.StringWidth(WorkerMetadataLine(msg, width)), width)
	}
}

func TestWorkerMetadataDoesNotInventRecordedEffort(t *testing.T) {
	name, effort := WorkerMetadata(nil)
	require.Equal(t, "—", name)
	require.Equal(t, "—", effort)
	name, effort = WorkerMetadata(&message.Message{Model: models.ModelID("unknown/worker")})
	require.Equal(t, "unknown/worker", name)
	require.Equal(t, "—", effort)
}
