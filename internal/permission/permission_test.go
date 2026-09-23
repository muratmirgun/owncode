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

func TestApprovedWorkerBypassesPendingPrompt(t *testing.T) {
	service := NewPermissionService()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	events := service.Subscribe(ctx)
	done := make(chan bool, 1)
	go func() {
		done <- Request(ctx, service, CreatePermissionRequest{SessionID: "pending", Path: "/workspace/file"})
	}()
	first := <-events
	service.AutoApproveSession("approved")
	require.True(t, Request(ctx, service, CreatePermissionRequest{SessionID: "approved", Path: "/workspace/file"}))
	service.Deny(first.Payload)
	require.False(t, <-done)
}

func TestYOLOTogglesPendingAndFuturePermissions(t *testing.T) {
	service := NewPermissionService()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	events := service.Subscribe(ctx)
	require.False(t, YOLOEnabled(service))
	done := make(chan bool, 1)
	go func() {
		done <- Request(ctx, service, CreatePermissionRequest{SessionID: "worker", Path: "/workspace/file"})
	}()
	require.Equal(t, pubsub.CreatedEvent, (<-events).Type)
	require.True(t, SetYOLO(service, true))
	require.True(t, <-done)
	require.Equal(t, pubsub.DeletedEvent, (<-events).Type)
	require.True(t, Request(ctx, service, CreatePermissionRequest{SessionID: "another-worker", Path: "/workspace/file"}))
	stopped, stop := context.WithCancel(ctx)
	stop()
	require.False(t, Request(stopped, service, CreatePermissionRequest{SessionID: "canceled", Path: "/workspace/file"}))
	require.True(t, SetYOLO(service, false))
	go func() {
		done <- Request(ctx, service, CreatePermissionRequest{SessionID: "worker", Path: "/workspace/file"})
	}()
	next := <-events
	require.Equal(t, pubsub.CreatedEvent, next.Type)
	service.Deny(next.Payload)
	require.False(t, <-done)
	require.False(t, YOLOEnabled(NewPermissionService()))
}
