package cmd

import (
	"strings"
	"testing"

	"github.com/muratmirgun/owncode/internal/session"
)

func TestExitSummaryResumeCommand(t *testing.T) {
	t.Parallel()
	saved := session.Session{ID: "session-123", Title: "Inspect scroll handling"}
	output := exitSummary(saved, "/tmp/project's folder", 100, false)
	if !strings.Contains(output, "Session   Inspect scroll handling") || !strings.Contains(output, `owncode -c '/tmp/project'"'"'s folder' -s session-123`) {
		t.Fatalf("unexpected exit summary: %q", output)
	}
	if strings.Contains(output, "\x1b") {
		t.Fatal("plain output contains terminal escapes")
	}
	if output := exitSummary(session.Session{}, "", 80, false); output != "" {
		t.Fatalf("empty session produced %q", output)
	}
	narrow := exitSummary(saved, "", 35, false)
	if !strings.Contains(narrow, "OwnCode") || !strings.Contains(narrow, "owncode -s session-123") {
		t.Fatalf("narrow summary lost logo or command: %q", narrow)
	}
}

func TestExitSummarySanitizesTitle(t *testing.T) {
	t.Parallel()
	output := exitSummary(session.Session{ID: "session-123", Title: "hello\x1b[31m\nworld\x07"}, "", 80, false)
	if strings.ContainsAny(output, "\x1b\x07") || !strings.Contains(output, "hello world") {
		t.Fatalf("unsafe title output: %q", output)
	}
}
