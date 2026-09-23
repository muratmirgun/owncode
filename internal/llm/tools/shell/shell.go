package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/muratmirgun/owncode/internal/config"
)

// PersistentShell serializes commands within one session, not across workers.
// The shell process and its children share a group for cancellation and cleanup.
type PersistentShell struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	gate  chan struct{}
	done  chan struct{}
	stop  sync.Once
}

var shellPool = struct {
	sync.Mutex
	items  map[string]*PersistentShell
	closed bool
}{items: make(map[string]*PersistentShell)}

// GetSessionShell returns a persistent shell owned by one conversation or worker.
func GetSessionShell(sessionID, workingDir string) (*PersistentShell, error) {
	shellPool.Lock()
	defer shellPool.Unlock()
	if shellPool.closed {
		return nil, fmt.Errorf("shell service is closed")
	}
	if s := shellPool.items[sessionID]; s != nil {
		select {
		case <-s.done:
			s.Close()
		default:
			return s, nil
		}
	}
	s, err := newPersistentShell(workingDir)
	if err != nil {
		return nil, err
	}
	shellPool.items[sessionID] = s
	return s, nil
}

// CloseSessionShell releases a worker's process after its task ends.
func CloseSessionShell(sessionID string) {
	shellPool.Lock()
	s := shellPool.items[sessionID]
	delete(shellPool.items, sessionID)
	shellPool.Unlock()
	if s != nil {
		s.Close()
	}
}

// CloseAll stops the shells and rejects new commands during application shutdown.
func CloseAll() {
	shellPool.Lock()
	shellPool.closed = true
	items := shellPool.items
	shellPool.items = make(map[string]*PersistentShell)
	shellPool.Unlock()
	for _, s := range items {
		s.Close()
	}
}

func newPersistentShell(cwd string) (*PersistentShell, error) {
	var shellPath string
	var shellArgs []string
	if cfg := config.Get(); cfg != nil {
		shellPath, shellArgs = cfg.Shell.Path, cfg.Shell.Args
	}
	if shellPath == "" {
		shellPath = os.Getenv("SHELL")
	}
	if shellPath == "" {
		shellPath = "/bin/bash"
	}
	if len(shellArgs) == 0 {
		shellArgs = []string{"-l"}
	}
	cmd := exec.Command(shellPath, shellArgs...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	s := &PersistentShell{cmd: cmd, stdin: stdin, gate: make(chan struct{}, 1), done: make(chan struct{})}
	go func() {
		// Wait always reaps the process. Exec reports premature termination.
		_ = cmd.Wait()
		s.terminate()
		close(s.done)
	}()
	return s, nil
}

// Exec retains shell state between commands. The timeout includes queue time.
// Cancellation stops this session's process group and returns without draining
// another worker's queue. A later call can create a fresh shell.
func (s *PersistentShell) Exec(ctx context.Context, command string, timeoutMs int) (string, string, int, bool, error) {
	if timeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	case <-ctx.Done():
		return "", "", 143, true, ctx.Err()
	case <-s.done:
		return "", "", 1, false, fmt.Errorf("shell is not alive")
	}
	if err := ctx.Err(); err != nil {
		return "", "", 143, true, err
	}
	select {
	case <-s.done:
		return "", "", 1, false, fmt.Errorf("shell is not alive")
	default:
	}
	dir, err := os.MkdirTemp("", "owncode-shell-")
	if err != nil {
		return "", "", 1, false, err
	}
	defer os.RemoveAll(dir)
	stdoutFile := filepath.Join(dir, "stdout")
	stderrFile := filepath.Join(dir, "stderr")
	statusFile := filepath.Join(dir, "status")
	// Write the status atomically so the reader never sees an incomplete code.
	script := fmt.Sprintf("eval %s < /dev/null > %s 2> %s\n__owncode_exit=$?\nprintf '%%s' \"$__owncode_exit\" > %s\ncommand mv %s %s\n",
		shellQuote(command), shellQuote(stdoutFile), shellQuote(stderrFile), shellQuote(statusFile+".tmp"), shellQuote(statusFile+".tmp"), shellQuote(statusFile))
	if _, err := io.WriteString(s.stdin, script); err != nil {
		return "", "", 1, false, err
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Close()
			return readFileOrEmpty(stdoutFile), readFileOrEmpty(stderrFile), 143, true, ctx.Err()
		case <-s.done:
			return readFileOrEmpty(stdoutFile), readFileOrEmpty(stderrFile), 1, false, fmt.Errorf("shell exited before command completion")
		case <-ticker.C:
			status, err := os.ReadFile(statusFile)
			if err != nil {
				continue
			}
			var code int
			if _, err := fmt.Sscanf(string(status), "%d", &code); err != nil {
				return "", "", 1, false, err
			}
			return readFileOrEmpty(stdoutFile), readFileOrEmpty(stderrFile), code, false, nil
		}
	}
}

// Close terminates the process group and waits for the shell reaper.
func (s *PersistentShell) Close() {
	s.terminate()
	<-s.done
}

func (s *PersistentShell) terminate() {
	s.stop.Do(func() {
		// The process may already have exited. Cleanup is idempotent.
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		_ = s.stdin.Close()
	})
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func readFileOrEmpty(path string) string {
	content, _ := os.ReadFile(path)
	return string(content)
}
