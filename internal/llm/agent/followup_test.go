package agent

import (
	"testing"

	"github.com/muratmirgun/owncode/internal/message"
)

func TestWorkerInboxAndID(t *testing.T) {
	box := &taskInbox{}
	taskInboxes.Store("worker", box)
	defer taskInboxes.Delete("worker")
	if err := SteerTask("worker", "Inspect tests too"); err != nil {
		t.Fatal(err)
	}
	if got := box.next(); got != "Inspect tests too" {
		t.Fatal(got)
	}
	if got := box.next(); got != "" {
		t.Fatal(got)
	}
	if err := SteerTask("worker", "late"); err == nil {
		t.Fatal("late prompt lost silently")
	}
	if got := TaskID(message.ToolCall{ID: "second", Input: `{"worker_id":"first"}`}); got != "first" {
		t.Fatal(got)
	}
}

func TestWorkerMustHaveLaunchRecord(t *testing.T) {
	history := []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{Name: AgentToolName, ID: "worker"}}}}
	if !ownsWorker(history, "worker") || ownsWorker(history, "title-parent") {
		t.Fatal("worker ownership failed")
	}
}
