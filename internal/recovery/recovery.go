// Package recovery stores turn checkpoints and restores only unchanged turn effects.
package recovery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Service coordinates disk restoration and conversation visibility.
type Service struct {
	db      *sql.DB
	workdir string
	mu      sync.Mutex
}

// New creates a checkpoint service. Git is required for file discovery.
func New(conn *sql.DB, workdir string) *Service { return &Service{db: conn, workdir: workdir} }

type file struct {
	Data []byte
	Mode uint32
}
type snapshot struct {
	Root               string
	Files              map[string]file
	Messages           []string
	Summary            string
	Prompt, Completion int64
}

// Preview contains the paths and direction that the user must review.
type Preview struct {
	ID                 int64
	Session, Direction string
	Paths              []string
	Before, After      snapshot
}

func (s *Service) snapshot(ctx context.Context, session string) (snapshot, error) {
	result := snapshot{Files: map[string]file{}, Messages: []string{}}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", s.workdir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return result, fmt.Errorf("checkpoints require a Git working tree")
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return result, err
	}
	result.Root = root
	out, err = exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return result, err
	}
	paths := strings.Split(string(out), "\x00")
	if len(paths) > 20000 {
		return result, fmt.Errorf("checkpoint file limit exceeded")
	}
	confined, err := os.OpenRoot(root)
	if err != nil {
		return result, err
	}
	defer func() { _ = confined.Close() }() // Best-effort cleanup; preserve the primary result.
	// Never checkpoint the live SQLite database or its state directory.
	var databaseID int
	var databaseName, databaseFile string
	if err := s.db.QueryRowContext(ctx, `PRAGMA database_list`).Scan(&databaseID, &databaseName, &databaseFile); err != nil {
		return result, err
	}
	databaseFile = filepath.Clean(databaseFile)
	stateDir, err := filepath.EvalSymlinks(filepath.Dir(databaseFile))
	if err != nil {
		return result, err
	}
	databaseFile = filepath.Join(stateDir, filepath.Base(databaseFile))
	stateRelative, err := filepath.Rel(root, stateDir)
	if err != nil {
		return result, err
	}
	total := 0
	for _, path := range paths {
		absolute := filepath.Join(root, path)
		if absolute == databaseFile || absolute == databaseFile+"-wal" || absolute == databaseFile+"-shm" {
			continue
		}
		if stateRelative != "." && filepath.IsLocal(stateRelative) && (path == stateRelative || strings.HasPrefix(path, stateRelative+string(filepath.Separator))) {
			continue
		}
		if path == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !filepath.IsLocal(path) {
			return result, fmt.Errorf("invalid Git path")
		}
		info, err := confined.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return result, err
		}
		// Symlinks and submodules cannot be restored as ordinary files.
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > 8<<20 {
			return result, fmt.Errorf("checkpoint file exceeds 8 MiB: %s", path)
		}
		f, err := confined.Open(path)
		if err != nil {
			return result, err
		}
		data, readErr := io.ReadAll(io.LimitReader(f, (8<<20)+1))
		_ = f.Close() // The read/transaction result determines success.
		if readErr != nil {
			return result, readErr
		}
		total += len(data)
		if total > 32<<20 {
			return result, fmt.Errorf("checkpoint exceeds 32 MiB")
		}
		result.Files[path] = file{Data: data, Mode: uint32(info.Mode().Perm())}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM messages WHERE session_id=? AND NOT EXISTS (SELECT 1 FROM hidden_messages h WHERE h.message_id=messages.id) ORDER BY rowid`, session)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close() // The read/transaction result determines success.
			return result, err
		}
		result.Messages = append(result.Messages, id)
	}
	err = rows.Err()
	_ = rows.Close() // The read/transaction result determines success.
	if err != nil {
		return result, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(summary_message_id,''),prompt_tokens,completion_tokens FROM sessions WHERE id=?`, session).Scan(&result.Summary, &result.Prompt, &result.Completion)
	return result, err
}

// Begin records a baseline before a turn. It invalidates redo branches only after capture succeeds.
func (s *Service) Begin(ctx context.Context, session string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, err := s.snapshot(ctx, session)
	if err != nil {
		return 0, err
	}
	data, err := json.Marshal(before)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }() // Best-effort cleanup; preserve the primary result.
	if _, err = tx.ExecContext(ctx, `UPDATE turn_checkpoints SET state='abandoned' WHERE session_id=? AND state='undone'`, session); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO turn_checkpoints(session_id,state,before_state) VALUES(?,'recording',?)`, session, data)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// Finish records the resulting files and messages after a turn ends, including cancellation.
func (s *Service) Finish(ctx context.Context, id int64, session string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	after, err := s.snapshot(ctx, session)
	if err != nil {
		return err
	}
	var beforeData []byte
	if err := s.db.QueryRowContext(ctx, `SELECT before_state FROM turn_checkpoints WHERE id=? AND state='recording' AND session_id=?`, id, session).Scan(&beforeData); err != nil {
		return err
	}
	var before snapshot
	if err := json.Unmarshal(beforeData, &before); err != nil {
		return err
	}
	// Completed checkpoints retain only changed files, not a full repository copy.
	for path, old := range before.Files {
		next, ok := after.Files[path]
		if ok && old.Mode == next.Mode && bytes.Equal(old.Data, next.Data) {
			delete(before.Files, path)
			delete(after.Files, path)
		}
	}
	beforeData, err = json.Marshal(before)
	if err != nil {
		return err
	}
	data, err := json.Marshal(after)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // Best-effort cleanup; preserve the primary result.
	if _, err = tx.ExecContext(ctx, `UPDATE turn_checkpoints SET before_state=?,after_state=?,state='ready' WHERE id=? AND state='recording'`, beforeData, data, id); err != nil {
		return err
	}
	// Bound disk growth. Keep ten completed turns per session and all uncertain restores.
	if _, err = tx.ExecContext(ctx, `DELETE FROM turn_checkpoints WHERE session_id=? AND state NOT IN ('applying','recording') AND id NOT IN (SELECT id FROM turn_checkpoints WHERE session_id=? ORDER BY id DESC LIMIT 10)`, session, session); err != nil {
		return err
	}
	err = tx.Commit()
	return err
}

// Inspect selects the next undo or redo checkpoint without changing files.
func (s *Service) Inspect(ctx context.Context, session, direction string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inspect(ctx, session, direction)
}
func (s *Service) inspect(ctx context.Context, session, direction string) (Preview, error) {
	p := Preview{Session: session, Direction: direction}
	state, order := "ready", "DESC"
	if direction == "redo" {
		state, order = "undone", "ASC"
	} else if direction != "undo" {
		return p, fmt.Errorf("invalid recovery direction")
	}
	var uncertain int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM turn_checkpoints WHERE state='applying'`).Scan(&uncertain); err != nil {
		return p, err
	}
	if uncertain > 0 {
		return p, fmt.Errorf("an interrupted restoration needs manual inspection before another restore")
	}
	var before, after []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,before_state,after_state FROM turn_checkpoints WHERE session_id=? AND state=? ORDER BY id `+order+` LIMIT 1`, session, state).Scan(&p.ID, &before, &after)
	if err == sql.ErrNoRows {
		return p, fmt.Errorf("no %s checkpoint available", direction)
	}
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(before, &p.Before); err != nil {
		return p, err
	}
	if err = json.Unmarshal(after, &p.After); err != nil {
		return p, err
	}
	if p.Before.Root != p.After.Root {
		return p, fmt.Errorf("checkpoint roots differ")
	}
	for path, old := range p.Before.Files {
		next, ok := p.After.Files[path]
		if !ok || old.Mode != next.Mode || !bytes.Equal(old.Data, next.Data) {
			p.Paths = append(p.Paths, path)
		}
	}
	for path := range p.After.Files {
		if _, ok := p.Before.Files[path]; !ok {
			p.Paths = append(p.Paths, path)
		}
	}
	sort.Strings(p.Paths)
	return p, nil
}

func readCurrent(root *os.Root, path string) (file, bool, error) {
	info, err := root.Lstat(path)
	if os.IsNotExist(err) {
		return file{}, false, nil
	}
	if err != nil {
		return file{}, false, err
	}
	if !info.Mode().IsRegular() {
		return file{}, false, fmt.Errorf("changed path is no longer a regular file: %s", path)
	}
	f, err := root.Open(path)
	if err != nil {
		return file{}, false, err
	}
	defer func() { _ = f.Close() }() // Best-effort cleanup; preserve the primary result.
	data, err := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	return file{Data: data, Mode: uint32(info.Mode().Perm())}, true, err
}
func writeFile(root *os.Root, path string, value file, exists bool) error {
	if !exists {
		err := root.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp := path + fmt.Sprintf(".owncode-restore-%d", time.Now().UnixNano())
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(value.Mode))
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temp) }() // Best-effort cleanup; preserve the primary result.
	if _, err = f.Write(value.Data); err != nil {
		_ = f.Close() // The read/transaction result determines success.
		return err
	}
	if err = f.Chmod(os.FileMode(value.Mode)); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return root.Rename(temp, path)
}

// Apply rechecks the preview and rejects later edits before restoring the whole turn.
func (s *Service) Apply(ctx context.Context, selected Preview) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.inspect(ctx, selected.Session, selected.Direction)
	if err != nil {
		return err
	}
	if p.ID != selected.ID {
		return fmt.Errorf("checkpoint changed; inspect again")
	}
	expected, target := p.After, p.Before
	state := "undone"
	if p.Direction == "redo" {
		expected, target = p.Before, p.After
		state = "ready"
	}
	var incomplete int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM turn_checkpoints WHERE session_id=? AND state='recording' AND id>=?`, p.Session, p.ID).Scan(&incomplete); err != nil {
		return err
	}
	if incomplete > 0 {
		return fmt.Errorf("a newer turn has no complete checkpoint")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM messages WHERE session_id=? AND NOT EXISTS(SELECT 1 FROM hidden_messages h WHERE h.message_id=messages.id) ORDER BY rowid`, p.Session)
	if err != nil {
		return err
	}
	visible := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close() // The read/transaction result determines success.
			return err
		}
		visible = append(visible, id)
	}
	err = rows.Err()
	_ = rows.Close() // The read/transaction result determines success.
	if err != nil {
		return err
	}
	if !slices.Equal(visible, expected.Messages) {
		return fmt.Errorf("conversation changed after this checkpoint")
	}
	root, err := os.OpenRoot(expected.Root)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }() // Best-effort cleanup; preserve the primary result.
	for _, path := range p.Paths {
		current, exists, err := readCurrent(root, path)
		if err != nil {
			return err
		}
		want, had := expected.Files[path]
		if exists != had || current.Mode != want.Mode || !bytes.Equal(current.Data, want.Data) {
			return fmt.Errorf("file changed after the checkpoint: %s", path)
		}
	}
	// Keep a durable marker so a process crash cannot silently permit a second restoration.
	oldState := "ready"
	if p.Direction == "redo" {
		oldState = "undone"
	}
	claimed, err := s.db.ExecContext(ctx, `UPDATE turn_checkpoints SET state='applying' WHERE id=? AND state=? AND NOT EXISTS(SELECT 1 FROM turn_checkpoints WHERE state='applying')`, p.ID, oldState)
	if err != nil {
		return err
	}
	count, err := claimed.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("another restoration changed this checkpoint")
	}
	changed := []string{}
	rollback := func() error {
		for i := len(changed) - 1; i >= 0; i-- {
			value, exists := expected.Files[changed[i]]
			if err := writeFile(root, changed[i], value, exists); err != nil {
				return err
			}
		}
		oldState := "ready"
		if p.Direction == "redo" {
			oldState = "undone"
		}
		_, err := s.db.ExecContext(context.WithoutCancel(ctx), `UPDATE turn_checkpoints SET state=? WHERE id=?`, oldState, p.ID)
		return err
	}
	for _, path := range p.Paths {
		value, exists := target.Files[path]
		if err = writeFile(root, path, value, exists); err != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				return fmt.Errorf("restore failed: %v; rollback failed: %w", err, rollbackErr)
			}
			return err
		}
		changed = append(changed, path)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Join(err, rollback())
	}
	defer func() { _ = tx.Rollback() }() // Best-effort cleanup; preserve the primary result.
	keep := map[string]bool{}
	for _, id := range p.Before.Messages {
		keep[id] = true
	}
	for _, id := range p.After.Messages {
		if keep[id] {
			continue
		}
		if p.Direction == "undo" {
			_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO hidden_messages(message_id) VALUES(?)`, id)
		} else {
			_, err = tx.ExecContext(ctx, `DELETE FROM hidden_messages WHERE message_id=?`, id)
		}
		if err != nil {
			_ = tx.Rollback() // The read/transaction result determines success.
			return errors.Join(err, rollback())
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET summary_message_id=NULLIF(?,''),prompt_tokens=?,completion_tokens=?,message_count=(SELECT count(*) FROM messages WHERE session_id=? AND NOT EXISTS(SELECT 1 FROM hidden_messages h WHERE h.message_id=messages.id)) WHERE id=?`, target.Summary, target.Prompt, target.Completion, p.Session, p.Session)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE turn_checkpoints SET state=? WHERE id=?`, state, p.ID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		_ = tx.Rollback() // The read/transaction result determines success.
		err = errors.Join(err, rollback())
	}
	return err
}

// Diff returns the exact text changes proposed for one path.
func (p Preview) Diff(path string) (string, string) {
	before, after := p.After, p.Before
	if p.Direction == "redo" {
		before, after = p.Before, p.After
	}
	return string(before.Files[path].Data), string(after.Files[path].Data)
}
