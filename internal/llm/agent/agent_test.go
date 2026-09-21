package agent

import (
	"context"
	"errors"
	"github.com/muratmirgun/owncode/internal/llm/provider"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"testing"
	"testing/synctest"

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

type terminalTestProvider struct{ provider.Provider }

func (terminalTestProvider) Model() models.Model { return models.Model{ID: "fixture"} }

type terminalTestMessages struct{ message.Service }

func (terminalTestMessages) List(context.Context, string) ([]message.Message, error) {
	return nil, errors.New("fixture storage failure")
}

func TestRunCompletesWithoutResultReader(t *testing.T) {
	// synctest fails if Run leaves a goroutine blocked on its unconsumed result.
	synctest.Test(t, func(t *testing.T) {
		a := &agent{Broker: pubsub.NewBroker[AgentEvent](), provider: terminalTestProvider{}, messages: terminalTestMessages{}, name: config.AgentTask}
		defer a.Shutdown()
		results, err := a.Run(t.Context(), "session", "hello")
		require.NoError(t, err)
		synctest.Wait()
		require.False(t, a.IsBusy())
		require.Len(t, results, 1)
		require.Error(t, (<-results).Error)
		_, open := <-results
		require.False(t, open)
	})
}
