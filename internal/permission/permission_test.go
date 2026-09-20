package permission

import (
	"context"
	"testing"
	"time"

	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestQueuedApprovalAndCancellation(t *testing.T) {
	service := NewPermissionService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := service.Subscribe(ctx)
	firstCtx, cancelFirst := context.WithCancel(ctx)
	firstDone := make(chan bool, 1)
	go func() {
		firstDone <- Request(firstCtx, service, CreatePermissionRequest{SessionID: "first", ToolName: "edit", Path: "/workspace/file"})
	}()
	first := <-events
	require.Equal(t, pubsub.CreatedEvent, first.Type)
	secondDone := make(chan bool, 1)
	go func() {
		secondDone <- Request(ctx, service, CreatePermissionRequest{SessionID: "second", ToolName: "edit", Path: "/workspace/file"})
	}()
	cancelFirst()
	require.False(t, <-firstDone)
	deleted := <-events
	require.Equal(t, pubsub.DeletedEvent, deleted.Type)
	second := <-events
	require.Equal(t, pubsub.CreatedEvent, second.Type)
	require.Equal(t, "second", second.Payload.SessionID)
	service.Grant(second.Payload)
	require.True(t, <-secondDone)
	service.Grant(first.Payload) // A late response cannot block or approve another worker.
	service.Deny(second.Payload)
}
