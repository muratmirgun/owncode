package tools

import (
	"context"
	"errors"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/permission"
	"testing"
)

func TestAutomationDisabledAndDenied(t *testing.T) {
	previous := config.CurrentAutomation()
	t.Cleanup(func() { config.SetAutomationSnapshot(previous) })
	tool := NewAutomationTools(nil)[0]
	config.SetAutomationSnapshot(config.AutomationSettings{})
	result, err := tool.Run(context.Background(), ToolCall{Input: `{"action":"open","url":"https://example.com"}`})
	if err != nil || !result.IsError {
		t.Fatal("disabled tool did not return setup error")
	}
	config.SetAutomationSnapshot(config.AutomationSettings{Browser: "embedded"})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session")
	ctx = context.WithValue(ctx, MessageIDContextKey, "message")
	_, err = tool.Run(ctx, ToolCall{Input: `{"action":"open","url":"https://example.com"}`})
	if !errors.Is(err, permission.ErrorPermissionDenied) {
		t.Fatalf("missing permission service: %v", err)
	}
}
