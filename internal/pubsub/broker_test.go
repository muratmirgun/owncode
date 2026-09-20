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
