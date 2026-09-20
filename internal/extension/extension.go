// Package extension implements the version 1 metadata event protocol.
package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Config explicitly enables one trusted local process. It never loads from project config.
type Config struct {
	Enabled bool     `json:"enabled"`
	Command []string `json:"command"`
}

// Event contains metadata only. Prompts, responses, file content, and credentials are excluded.
type Event struct {
	Version   int    `json:"version"`
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	Outcome   string `json:"outcome"`
}

// Reply is the only accepted process output. It cannot request tool execution or change context.
type Reply struct {
	Version int    `json:"version"`
	Notice  string `json:"notice"`
}

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if len(data) > 8192-b.Len() {
		return 0, fmt.Errorf("extension output exceeds 8 KiB")
	}
	return b.Buffer.Write(data)
}

// Run sends one JSON event to stdin and reads one JSON reply from stdout.
func Run(ctx context.Context, cfg Config, event Event, workdir string) (Reply, error) {
	if !cfg.Enabled {
		return Reply{}, nil
	}
	if len(cfg.Command) == 0 || !filepath.IsAbs(cfg.Command[0]) {
		return Reply{}, fmt.Errorf("extension command must use an absolute executable path")
	}
	if event.Type != "turn.complete" {
		return Reply{}, fmt.Errorf("unsupported extension event")
	}
	event.Version = 1
	data, err := json.Marshal(event)
	if err != nil {
		return Reply{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	command.Dir = workdir
	command.WaitDelay = 100 * time.Millisecond
	// A hook receives a minimal environment. No model credentials are injected.
	for _, key := range []string{"PATH", "HOME", "LANG", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			command.Env = append(command.Env, key+"="+value)
		}
	}
	command.Stdin = bytes.NewReader(append(data, '\n'))
	var output boundedBuffer
	command.Stdout = &output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return Reply{}, fmt.Errorf("extension process failed: %w", err)
	}
	var reply Reply
	decoder := json.NewDecoder(&output)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reply); err != nil {
		return Reply{}, fmt.Errorf("extension reply: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Reply{}, fmt.Errorf("extension must return one JSON object")
	}
	if reply.Version != 1 || len(reply.Notice) > 512 {
		return Reply{}, fmt.Errorf("unsupported extension reply version or notice size")
	}
	return reply, nil
}
