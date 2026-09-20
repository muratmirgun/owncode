package message

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
)

const updateInterval = 33 * time.Millisecond

type pendingUpdate struct {
	sessionID string
	mu        sync.Mutex
	latest    Message
	dirty     bool
	timer     *time.Timer
	toolState map[string]bool
}

// Update batches stream deltas before persistence and publication. Finished
// messages and tool boundaries remain synchronous durability points.
func (s *service) Update(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("message storage is closed")
	}
	p := s.pending[msg.ID]
	if p == nil {
		p = &pendingUpdate{sessionID: msg.SessionID, toolState: make(map[string]bool)}
		s.pending[msg.ID] = p
	}
	s.updates.Add(1)
	s.mu.Unlock()
	defer s.updates.Done()
	p.mu.Lock()
	defer p.mu.Unlock()
	msg.Parts = slices.Clone(msg.Parts)
	p.latest, p.dirty = msg, true
	immediate := msg.Role != Assistant || msg.IsFinished()
	for _, call := range msg.ToolCalls() {
		finished, exists := p.toolState[call.ID]
		if !exists || finished != call.Finished {
			immediate = true
		}
		p.toolState[call.ID] = call.Finished
	}
	if immediate {
		return s.flushLocked(ctx, p)
	}
	if p.timer == nil {
		p.timer = time.AfterFunc(updateInterval, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// Failed writes remain dirty. The next update or explicit flush retries
			// them and returns any persistent error to the caller.
			_ = s.flushLocked(ctx, p)
		})
	}
	return nil
}

func (s *service) flushLocked(ctx context.Context, p *pendingUpdate) error {
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	if !p.dirty {
		return nil
	}
	if err := s.writeUpdate(ctx, p.latest); err != nil {
		return err
	}
	p.dirty = false
	// Release large completed snapshots while retaining the per-ID write lock.
	if p.latest.IsFinished() {
		p.latest.Parts = nil
		clear(p.toolState)
	}
	return nil
}

// Flush persists the latest accepted snapshot before a consistency-sensitive read.
func (s *service) Flush(ctx context.Context, id string) error {
	s.mu.Lock()
	p := s.pending[id]
	s.mu.Unlock()
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return s.flushLocked(ctx, p)
}

func (s *service) flushSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	items := make([]*pendingUpdate, 0, len(s.pending))
	for _, p := range s.pending {
		if sessionID == "" || p.sessionID == sessionID {
			items = append(items, p)
		}
	}
	s.mu.Unlock()
	var result error
	for _, p := range items {
		p.mu.Lock()
		result = errors.Join(result, s.flushLocked(ctx, p))
		p.mu.Unlock()
	}
	return result
}

// FlushAll persists pending snapshots, including updates from canceled streams.
func (s *service) FlushAll(ctx context.Context) error { return s.flushSession(ctx, "") }

// Close rejects new updates and drains every timer before application shutdown.
func (s *service) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.updates.Wait()
	return s.FlushAll(ctx)
}
