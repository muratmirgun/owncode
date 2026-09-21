package pubsub

import (
	"context"
	"sync"
	"testing"
)

func TestPublishDuringUnsubscribe(t *testing.T) {
	b := NewBroker[int]()
	var workers sync.WaitGroup
	for range 100 {
		ctx, cancel := context.WithCancel(context.Background())
		b.Subscribe(ctx)
		workers.Go(func() {
			defer cancel()
			for i := range 100 {
				b.Publish(UpdatedEvent, i)
			}
		})
	}
	workers.Wait()
	b.Shutdown()
}

// A stalled consumer must not stall five producers or lose lifecycle events.
func TestFiveProducersWithSlowConsumer(t *testing.T) {
	b := NewBrokerWithOptions[int](1, 1)
	defer b.Shutdown()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := b.Subscribe(ctx)
	var wg sync.WaitGroup
	for worker := range 5 {
		wg.Go(func() {
			for n := range 1000 {
				b.Publish(CreatedEvent, worker*1000+n)
			}
		})
	}
	wg.Wait()
	last := [5]int{-1, -1, -1, -1, -1}
	for range 5000 {
		event := <-events
		worker, n := event.Payload/1000, event.Payload%1000
		if n != last[worker]+1 {
			t.Fatalf("worker %d event %d after %d", worker, n, last[worker])
		}
		last[worker] = n
	}
	b.Shutdown()
	if b.GetSubscriberCount() != 0 {
		t.Fatal("subscribers remain after shutdown")
	}
}

func TestSnapshotQueuePreservesFinalAndLifecycle(t *testing.T) {
	b := NewSnapshotBroker(func(value int) string { return "message" })
	defer b.Shutdown()
	events := b.Subscribe(t.Context())
	b.Publish(CreatedEvent, 0)
	for i := 1; i <= 1000; i++ {
		b.Publish(UpdatedEvent, i)
	}
	b.Publish(DeletedEvent, 1001)
	last := -1
	for {
		event := <-events
		if event.Payload <= last {
			t.Fatalf("out of order: %d after %d", event.Payload, last)
		}
		if event.Type == DeletedEvent {
			if last != 1000 {
				t.Fatalf("lost final snapshot: got %d", last)
			}
			break
		}
		last = event.Payload
	}
}

func TestConcurrentShutdownAndSubscribe(t *testing.T) {
	b := NewBroker[int]()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { b.Subscribe(context.Background()); b.Shutdown() })
	}
	wg.Wait()
	if b.GetSubscriberCount() != 0 {
		t.Fatal("shutdown retained subscribers")
	}
}
