package extension

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProtocolAndLimits(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "hook")
	script := `#!/bin/sh
read event
case "$event" in *'"version":1'*'"type":"turn.complete"'*) printf '{"version":1,"notice":"done"}' ;; *) exit 4 ;; esac
`
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	reply, err := Run(t.Context(), Config{Enabled: true, Command: []string{path}}, Event{Type: "turn.complete"}, t.TempDir())
	if err != nil || reply.Notice != "done" {
		t.Fatalf("reply=%v err=%v", reply, err)
	}
	if _, err := Run(t.Context(), Config{Enabled: true, Command: []string{"relative"}}, Event{Type: "turn.complete"}, ""); err == nil {
		t.Fatal("relative command allowed")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '{\"version\":2}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(t.Context(), Config{Enabled: true, Command: []string{path}}, Event{Type: "turn.complete"}, ""); err == nil {
		t.Fatal("invalid version accepted")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 20\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := Run(ctx, Config{Enabled: true, Command: []string{path}}, Event{Type: "turn.complete"}, ""); err == nil {
		t.Fatal("timeout ignored")
	}
}
