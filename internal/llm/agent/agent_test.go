package agent

import (
	"context"
	"testing"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/stretchr/testify/require"
)

func TestUnconfiguredAgent(t *testing.T) {
	t.Parallel()
	// Nil dependencies ensure rejection happens before storage or provider access.
	a := &agent{}
	require.Empty(t, a.Model().ID)
	require.False(t, a.IsBusy())
	require.False(t, a.IsSessionBusy("session"))
	events, err := a.Run(context.Background(), "session", "hello")
	require.ErrorIs(t, err, ErrNotConfigured)
	require.Nil(t, events)
	require.ErrorIs(t, a.Summarize(context.Background(), "session"), ErrNotConfigured)
	_, err = a.Update(config.AgentCoder, models.GPT41)
	require.ErrorIs(t, err, ErrNotConfigured)
	a.Cancel("session")
	require.False(t, a.IsBusy())
}
