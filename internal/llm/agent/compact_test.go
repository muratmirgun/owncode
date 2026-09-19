package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/llm/provider"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
)

type compactProvider struct {
	provider.Provider
	received chan []message.Message
	block    bool
}

func (p compactProvider) Model() models.Model { return models.Model{ID: "test"} }
func (p compactProvider) SendMessages(ctx context.Context, msgs []message.Message, _ []tools.BaseTool) (*provider.ProviderResponse, error) {
	p.received <- msgs
	if p.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &provider.ProviderResponse{Content: "Summary", Usage: provider.TokenUsage{OutputTokens: 30}}, nil
}

type compactMessages struct{ message.Service }

func (compactMessages) List(context.Context, string) ([]message.Message, error) {
	return []message.Message{
		{ID: "old", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "obsolete"}}},
		{ID: "summary", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "previous summary"}}},
		{ID: "new", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "current work"}}},
	}, nil
}
func (compactMessages) Create(_ context.Context, _ string, params message.CreateMessageParams) (message.Message, error) {
	return message.Message{ID: "saved", Parts: params.Parts}, nil
}

type compactSessions struct {
	session.Service
	fail bool
}

func (compactSessions) Get(context.Context, string) (session.Session, error) {
	return session.Session{ID: "session", SummaryMessageID: "summary", PromptTokens: 900}, nil
}
func (s compactSessions) Save(_ context.Context, value session.Session) (session.Session, error) {
	if s.fail {
		return session.Session{}, errors.New("save failed")
	}
	return value, nil
}

func TestCompactUsesActiveContextAndSavedOptions(t *testing.T) {
	for _, fail := range []bool{false, true} {
		p := compactProvider{received: make(chan []message.Message, 1)}
		a := &agent{Broker: pubsub.NewBroker[AgentEvent](), provider: p, summarizeProvider: p, messages: compactMessages{}, sessions: compactSessions{fail: fail}}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		events := a.Subscribe(ctx)
		require.NoError(t, a.Summarize(ctx, "session", CompactOptions{Mode: "handoff", Focus: "Keep exact test commands"}))
		select {
		case msgs := <-p.received:
			require.Len(t, msgs, 3)
			require.Equal(t, "summary", msgs[0].ID)
			require.Equal(t, message.User, msgs[0].Role)
			prompt := msgs[2].Content().String()
			require.Contains(t, prompt, "structured handoff")
			require.Contains(t, prompt, "Keep exact test commands")
		case <-ctx.Done():
			t.Fatal("summary request timed out")
		}
		for {
			select {
			case event := <-events:
				require.Equal(t, "session", event.Payload.SessionID)
				if !event.Payload.Done {
					continue
				}
				if fail {
					require.ErrorContains(t, event.Payload.Error, "save failed")
				} else {
					require.NoError(t, event.Payload.Error)
					require.Contains(t, event.Payload.Progress, "900 → 30")
				}
				cancel()
			case <-ctx.Done():
				t.Fatal("summary completion timed out")
			}
			if ctx.Err() != nil {
				break
			}
		}
		a.Broker.Shutdown()
	}
}

func TestCompactCancellationKeepsSessionBusyUntilExit(t *testing.T) {
	p := compactProvider{received: make(chan []message.Message, 1), block: true}
	a := &agent{Broker: pubsub.NewBroker[AgentEvent](), provider: p, summarizeProvider: p, messages: compactMessages{}, sessions: compactSessions{}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	events := a.Subscribe(ctx)
	require.NoError(t, a.Summarize(ctx, "session"))
	select {
	case <-p.received:
	case <-ctx.Done():
		t.Fatal("request timed out")
	}
	require.True(t, a.IsSessionBusy("session"))
	require.ErrorIs(t, a.Summarize(ctx, "session"), ErrSessionBusy)
	a.Cancel("session")
	for {
		select {
		case event := <-events:
			if event.Payload.Done {
				require.ErrorIs(t, event.Payload.Error, context.Canceled)
				return
			}
		case <-ctx.Done():
			t.Fatal("cancellation timed out")
		}
	}
}

func TestActiveSummaryMessagesDoesNotMutateHistory(t *testing.T) {
	original := []message.Message{{ID: "summary", Role: message.Assistant}}
	active := activeSummaryMessages(original, "summary")
	require.Equal(t, message.User, active[0].Role)
	require.Equal(t, message.Assistant, original[0].Role)
	require.Error(t, (CompactOptions{Mode: "invalid"}).validate())
}

func TestCancelTaskOnlyCancelsSelectedAgent(t *testing.T) {
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	runningTasks.Store("first", cancelFirst)
	runningTasks.Store("second", cancelSecond)
	defer runningTasks.Delete("first")
	defer runningTasks.Delete("second")
	require.True(t, IsTaskRunning("first"))
	require.True(t, CancelTask("first"))
	require.ErrorIs(t, first.Err(), context.Canceled)
	require.NoError(t, second.Err())
	require.False(t, CancelTask("missing"))
}
