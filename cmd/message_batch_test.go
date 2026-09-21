package cmd

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/tui"
	"github.com/stretchr/testify/require"
)

func TestFourAgentBurstKeepsLatestSnapshots(t *testing.T) {
	var queue messageQueue
	for i := 0; i < 4; i++ {
		queue.add(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: message.Message{ID: fmt.Sprint(i)}})
	}
	for step := 0; step < 1000; step++ {
		for i := 0; i < 4; i++ {
			queue.add(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: message.Message{ID: fmt.Sprint(i), Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprint(step)}}}})
		}
	}
	events := queue.take()
	require.Len(t, events, 8)
	for _, event := range events[4:] {
		require.Equal(t, "999", event.Payload.Content().Text)
	}
	queue.add(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: message.Message{ID: "0"}})
	require.Len(t, queue.take(), 1)
}

func TestMessageBatchesFlushOnCloseAndCancel(t *testing.T) {
	input := make(chan pubsub.Event[message.Message], 1)
	input <- pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: message.Message{ID: "test"}}
	close(input)
	output := make(chan tea.Msg, 1)
	forwardMessageBatches(context.Background(), input, output)
	batch := (<-output).(tui.MessageBatchMsg)
	require.Len(t, batch, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { forwardMessageBatches(ctx, make(chan pubsub.Event[message.Message]), output); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("subscriber did not stop")
	}
}

func TestMessageBatchPreservesLifecycleAndFinalUsage(t *testing.T) {
	var q messageQueue
	msg := message.Message{ID: "a"}
	q.add(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
	q.add(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
	msg.AddFinish(message.FinishReasonEndTurn)
	msg.OutputTokens = 120
	q.add(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
	q.add(pubsub.Event[message.Message]{Type: pubsub.DeletedEvent, Payload: msg})
	events := q.take()
	require.Len(t, events, 3)
	require.True(t, events[1].Payload.IsFinished())
	require.Equal(t, int64(120), events[1].Payload.OutputTokens)
	require.Equal(t, pubsub.DeletedEvent, events[2].Type)
}

func TestSubscriberRetainsEventDuringLongUIStall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var workers sync.WaitGroup
		defer func() { cancel(); workers.Wait() }()
		input := make(chan pubsub.Event[int], 1)
		output := make(chan tea.Msg)
		input <- pubsub.Event[int]{Type: pubsub.CreatedEvent, Payload: 42}
		setupSubscriber(ctx, &workers, "fixture", func(context.Context) <-chan pubsub.Event[int] { return input }, output)
		time.Sleep(3 * time.Second)
		select {
		case msg := <-output:
			require.Equal(t, 42, msg.(pubsub.Event[int]).Payload)
		default:
			t.Fatal("subscriber discarded the event while the UI was unavailable")
		}
	})
}
