package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testShell(t *testing.T, id string) *PersistentShell {
	t.Helper()
	t.Setenv("SHELL", "/bin/bash")
	s, err := GetSessionShell(id, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { CloseSessionShell(id) })
	return s
}

func TestSessionShellStateAndIsolation(t *testing.T) {
	a, b := testShell(t, "state-a"), testShell(t, "state-b")
	_, _, code, _, err := a.Exec(t.Context(), "export OWNCODE_TEST_VALUE='alpha beta'; mkdir child; cd child", 2000)
	require.NoError(t, err)
	require.Zero(t, code)
	out, _, _, _, err := a.Exec(t.Context(), "printf '%s:%s' \"$OWNCODE_TEST_VALUE\" \"${PWD##*/}\"", 2000)
	require.NoError(t, err)
	require.Equal(t, "alpha beta:child", out)
	out, stderr, code, _, err := b.Exec(t.Context(), "printf '%s' \"${OWNCODE_TEST_VALUE-unset}\"; printf 'error' >&2; false", 2000)
	require.NoError(t, err)
	require.Equal(t, "unset", out)
	require.Equal(t, "error", stderr)
	require.Equal(t, 1, code)
}

func TestBusyWorkerDoesNotBlockAnotherShell(t *testing.T) {
	a, b := testShell(t, "busy-a"), testShell(t, "busy-b")
	marker := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, _, _, _, err := a.Exec(ctx, "touch "+shellQuote(marker)+"; sleep 20", 30000); done <- err }()
	require.Eventually(t, func() bool { _, err := os.Stat(marker); return err == nil }, 2*time.Second, 5*time.Millisecond)
	out, _, _, _, err := b.Exec(t.Context(), "printf ready", 1000)
	require.NoError(t, err)
	require.Equal(t, "ready", out)
	// A canceled request queued behind the busy command must return immediately.
	queued, stop := context.WithCancel(t.Context())
	stop()
	_, _, _, _, err = a.Exec(queued, "touch should-not-exist", 1000)
	require.ErrorIs(t, err, context.Canceled)
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("canceled command did not stop")
	}
	// Canceling one shell leaves the other worker usable.
	out, _, _, _, err = b.Exec(t.Context(), "printf still-alive", 1000)
	require.NoError(t, err)
	require.Equal(t, "still-alive", out)
}

func TestShellTimeoutAndRestart(t *testing.T) {
	s := testShell(t, "restart")
	_, _, _, interrupted, err := s.Exec(t.Context(), "sleep 20", 30)
	require.True(t, interrupted)
	require.True(t, errors.Is(err, context.DeadlineExceeded))
	next, err := GetSessionShell("restart", t.TempDir())
	require.NoError(t, err)
	require.NotSame(t, s, next)
	out, _, _, _, err := next.Exec(t.Context(), "printf restarted", 1000)
	require.NoError(t, err)
	require.Equal(t, "restarted", out)
}

func BenchmarkShellWorkers(b *testing.B) {
	for _, shared := range []bool{true, false} {
		b.Run(fmt.Sprintf("shared=%t", shared), func(b *testing.B) {
			b.Setenv("SHELL", "/bin/bash")
			dir := b.TempDir()
			shells := make([]*PersistentShell, 3)
			for i := range shells {
				id := fmt.Sprintf("bench-%t-%d", shared, i)
				if shared {
					id = "bench-shared"
				}
				var err error
				shells[i], err = GetSessionShell(id, dir)
				if err != nil {
					b.Fatal(err)
				}
				b.Cleanup(func() { CloseSessionShell(id) })
			}
			b.ResetTimer()
			for b.Loop() {
				done := make(chan error, 3)
				for _, s := range shells {
					go func() { _, _, _, _, err := s.Exec(b.Context(), "sleep 0.03", 1000); done <- err }()
				}
				for range shells {
					if err := <-done; err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
