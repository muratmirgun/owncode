// Package question coordinates typed questions between tools and the terminal.
package question

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/muratmirgun/owncode/internal/pubsub"
)

// Request describes one question. The user can always supply free text.
type Request struct {
	ID, SessionID, Text string
	Options             []string
}

// Answer contains a selected option or a free-text response.
type Answer struct {
	Text      string
	Dismissed bool
}

// Service owns pending replies. No UI means immediate failure, not an indefinite wait.
type Service struct {
	*pubsub.Broker[Request]
	mu      sync.Mutex
	pending map[string]chan Answer
}

// New creates an empty question service.
func New() *Service {
	return &Service{Broker: pubsub.NewBroker[Request](), pending: map[string]chan Answer{}}
}

// Ask waits for a user reply or caller cancellation.
func (s *Service) Ask(ctx context.Context, request Request) (Answer, error) {
	if err := ctx.Err(); err != nil {
		return Answer{}, err
	}
	if s == nil || s.GetSubscriberCount() == 0 {
		return Answer{}, fmt.Errorf("questions require an interactive terminal; request the missing information in the final response")
	}
	if strings.TrimSpace(request.Text) == "" || len(request.Text) > 2000 || len(request.Options) > 6 {
		return Answer{}, fmt.Errorf("question requires 1–2000 bytes and at most six options")
	}
	for _, option := range request.Options {
		if strings.TrimSpace(option) == "" || len(option) > 160 {
			return Answer{}, fmt.Errorf("each option requires 1–160 bytes")
		}
	}
	request.ID = uuid.NewString()
	request.Options = append([]string(nil), request.Options...)
	reply := make(chan Answer, 1)
	s.mu.Lock()
	s.pending[request.ID] = reply
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, request.ID)
		s.mu.Unlock()
		s.Publish(pubsub.DeletedEvent, request)
	}()
	s.Publish(pubsub.CreatedEvent, request)
	select {
	case <-ctx.Done():
		return Answer{}, ctx.Err()
	case answer := <-reply:
		return answer, nil
	}
}

// Reply resolves a pending question once. It never blocks the terminal loop.
func (s *Service) Reply(id string, answer Answer) bool {
	if len(answer.Text) > 4096 || (!answer.Dismissed && strings.TrimSpace(answer.Text) == "") {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.pending[id]
	if !ok {
		return false
	}
	delete(s.pending, id)
	ch <- answer
	return true
}
