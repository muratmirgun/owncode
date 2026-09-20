package cmd

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/tui"
)

type messageQueue struct {
	events  tui.MessageBatchMsg
	updates map[string]int
}

func (q *messageQueue) add(event pubsub.Event[message.Message]) {
	if q.updates == nil {
		q.updates = make(map[string]int)
	}
	// Updated payloads are complete snapshots, so only the latest one is needed.
	// Keep creation/deletion events and do not merge across lifecycle boundaries.
	if event.Type == pubsub.UpdatedEvent {
		if index, ok := q.updates[event.Payload.ID]; ok {
			q.events[index] = event
			return
		}
		q.updates[event.Payload.ID] = len(q.events)
	} else {
		delete(q.updates, event.Payload.ID)
	}
	q.events = append(q.events, event)
}

func (q *messageQueue) take() tui.MessageBatchMsg {
	events := q.events
	q.events = nil
	clear(q.updates)
	return events
}

func forwardMessageBatches(ctx context.Context, input <-chan pubsub.Event[message.Message], output chan<- tea.Msg) {
	// Bound full-screen redraws across all agents, not separately per agent.
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var queue messageQueue
	flush := func() bool {
		if len(queue.events) == 0 {
			return true
		}
		select {
		case output <- queue.take():
			return true
		case <-ctx.Done():
			return false
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-input:
			if !ok {
				flush()
				return
			}
			queue.add(event)
		case <-ticker.C:
			if !flush() {
				return
			}
		}
	}
}
