package pubsub

import (
	"context"
	"sync"
)

const bufferSize = 64

type subscription[T any] struct {
	mu      sync.Mutex
	queue   []Event[T]
	updates map[string]int
	wake    chan struct{}
	output  chan Event[T]
	closed  bool
}

// Broker preserves lifecycle order without holding producers behind a slow UI.
// Lifecycle events are retained until consumed or the subscription ends.
// Snapshot brokers coalesce pending updates to bound streaming backlogs.
type Broker[T any] struct {
	mu                sync.Mutex
	subs              map[*subscription[T]]struct{}
	done              chan struct{}
	once              sync.Once
	workers           sync.WaitGroup
	channelBufferSize int
	key               func(T) string
}

func NewBroker[T any]() *Broker[T] { return NewBrokerWithOptions[T](bufferSize, 1000) }

// NewSnapshotBroker coalesces complete UpdatedEvent snapshots by identity.
// Creation and deletion events always form ordering boundaries.
func NewSnapshotBroker[T any](key func(T) string) *Broker[T] {
	b := NewBroker[T]()
	b.key = key
	return b
}

// NewBrokerWithOptions configures the delivery buffer. maxEvents is retained for
// API compatibility; it cannot be a drop limit for reliable lifecycle events.
func NewBrokerWithOptions[T any](channelBufferSize, maxEvents int) *Broker[T] {
	return &Broker[T]{subs: make(map[*subscription[T]]struct{}), done: make(chan struct{}), channelBufferSize: max(0, channelBufferSize)}
}

func (b *Broker[T]) Shutdown() {
	b.mu.Lock()
	b.once.Do(func() { close(b.done) })
	b.mu.Unlock()
	b.workers.Wait()
}

func (b *Broker[T]) Subscribe(ctx context.Context) <-chan Event[T] {
	s := &subscription[T]{wake: make(chan struct{}, 1), output: make(chan Event[T], b.channelBufferSize), updates: make(map[string]int)}
	b.mu.Lock()
	select {
	case <-b.done:
		close(s.output)
		b.mu.Unlock()
		return s.output
	default:
	}
	b.subs[s] = struct{}{}
	b.workers.Add(1)
	b.mu.Unlock()
	go func() {
		defer b.workers.Done()
		defer func() {
			b.mu.Lock()
			delete(b.subs, s)
			b.mu.Unlock()
			s.mu.Lock()
			s.closed = true
			s.queue = nil
			clear(s.updates)
			s.mu.Unlock()
			close(s.output)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.done:
				return
			case <-s.wake:
			}
			for {
				s.mu.Lock()
				if len(s.queue) == 0 {
					s.mu.Unlock()
					break
				}
				// Queue indexes remain stable until a batch is drained.
				batch := s.queue
				s.queue = nil
				clear(s.updates)
				s.mu.Unlock()
				for i := range batch {
					event := batch[i]
					batch[i] = Event[T]{}
					select {
					case <-ctx.Done():
						return
					case <-b.done:
						return
					case s.output <- event:
					}
				}
			}
		}
	}()
	return s.output
}

func (b *Broker[T]) GetSubscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

func (b *Broker[T]) Publish(t EventType, payload T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-b.done:
		return
	default:
	}
	event := Event[T]{Type: t, Payload: payload}
	for s := range b.subs {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			continue
		}
		key := ""
		if b.key != nil {
			key = b.key(payload)
		}
		if key != "" && t == UpdatedEvent {
			if i, ok := s.updates[key]; ok {
				s.queue[i] = event
				s.mu.Unlock()
				continue
			}
			s.updates[key] = len(s.queue)
		} else {
			// Do not move a snapshot past a lifecycle boundary.
			clear(s.updates)
		}
		s.queue = append(s.queue, event)
		s.mu.Unlock()
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}
