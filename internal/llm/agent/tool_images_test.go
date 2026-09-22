package agent

import (
	"context"
	"github.com/muratmirgun/owncode/internal/message"
	"testing"
)

func TestToolImagesOnlyLatestAndVisionEnabled(t *testing.T) {
	history := []message.Message{{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{Image: []byte("old")}}}, {Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{Image: []byte("new")}}}}
	if got := withToolImages(history, false); len(got) != 2 {
		t.Fatal("text model received screenshots")
	}
	got := withToolImages(history, true)
	if len(got) != 3 {
		t.Fatalf("got %d messages", len(got))
	}
	images := got[2].BinaryContent()
	if len(images) != 1 || string(images[0].Data) != "new" {
		t.Fatal("incorrect image batch")
	}
	if len(history) != 2 || len(history[1].Parts) != 1 {
		t.Fatal("history mutated")
	}
}

func TestShakeArchivesToolScreenshots(t *testing.T) {
	original := []message.Message{{ID: "screenshot", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{Content: "Screenshot", Image: []byte("png-data")}}}}
	result, err := shakeContext(context.Background(), original, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.messages) != 1 || len(result.messages[0].ToolResults()[0].Image) != 0 {
		t.Fatal("shake retained the screenshot")
	}
	if len(original[0].ToolResults()[0].Image) == 0 {
		t.Fatal("shake mutated original history")
	}
}
