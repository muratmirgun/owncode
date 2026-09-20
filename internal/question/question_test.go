package question

import (
	"context"
	"testing"

	"github.com/muratmirgun/owncode/internal/pubsub"
)

func TestQuestionReplyAndCancellation(t *testing.T) {
	t.Parallel()
	service := New()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := service.Subscribe(ctx)
	done := make(chan Answer, 1)
	go func() {
		answer, err := service.Ask(ctx, Request{Text: "Choose", Options: []string{"One", "Two"}})
		if err != nil {
			t.Error(err)
		}
		done <- answer
	}()
	request := <-events
	if request.Type != pubsub.CreatedEvent {
		t.Fatal("missing question")
	}
	if !service.Reply(request.Payload.ID, Answer{Text: "custom answer"}) {
		t.Fatal("reply failed")
	}
	if service.Reply(request.Payload.ID, Answer{Text: "duplicate"}) {
		t.Fatal("duplicate reply accepted")
	}
	if answer := <-done; answer.Text != "custom answer" {
		t.Fatal(answer)
	}
	<-events // deletion
	ended := make(chan error, 1)
	go func() { _, err := service.Ask(ctx, Request{Text: "Cancel me"}); ended <- err }()
	<-events
	cancel()
	if err := <-ended; err != context.Canceled {
		t.Fatalf("cancel = %v", err)
	}
}
func TestQuestionNeedsInteractiveSubscriber(t *testing.T) {
	t.Parallel()
	if _, err := New().Ask(t.Context(), Request{Text: "Question"}); err == nil {
		t.Fatal("noninteractive question blocked")
	}
}
