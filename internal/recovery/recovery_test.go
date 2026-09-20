package recovery

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func setup(t *testing.T) (*Service, string) { return setupWithStateDir(t, false) }
func setupWithStateDir(t *testing.T, local bool) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	databaseDir := t.TempDir()
	if local {
		databaseDir = filepath.Join(root, ".owncode")
		if err := os.Mkdir(databaseDir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	conn, err := sql.Open("sqlite3", filepath.Join(databaseDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, query := range []string{
		`CREATE TABLE sessions(id TEXT PRIMARY KEY,summary_message_id TEXT,prompt_tokens INTEGER DEFAULT 0,completion_tokens INTEGER DEFAULT 0,message_count INTEGER DEFAULT 0)`,
		`CREATE TABLE messages(id TEXT PRIMARY KEY,session_id TEXT)`,
		`CREATE TABLE hidden_messages(message_id TEXT PRIMARY KEY)`,
		`CREATE TABLE turn_checkpoints(id INTEGER PRIMARY KEY AUTOINCREMENT,session_id TEXT,state TEXT,before_state BLOB,after_state BLOB)`,
		`INSERT INTO sessions(id) VALUES('session')`,
	} {
		if _, err := conn.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	return New(conn, root), root
}
func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestUndoRedoAndEditProtection(t *testing.T) {
	t.Parallel()
	service, root := setup(t)
	path := filepath.Join(root, "main.go")
	put(t, path, "before")
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	id, err := service.Begin(t.Context(), "session")
	if err != nil {
		t.Fatal(err)
	}
	put(t, path, "after")
	put(t, filepath.Join(root, "new.go"), "new")
	if _, err := service.db.Exec(`INSERT INTO messages(id,session_id) VALUES('message','session')`); err != nil {
		t.Fatal(err)
	}
	if err := service.Finish(t.Context(), id, "session"); err != nil {
		t.Fatal(err)
	}
	preview, err := service.Inspect(t.Context(), "session", "undo")
	if err != nil {
		t.Fatal(err)
	}
	put(t, path, "user edit")
	if err := service.Apply(t.Context(), preview); err == nil {
		t.Fatal("restored over a later edit")
	}
	put(t, path, "after")
	if err := service.Apply(t.Context(), preview); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0666 {
		t.Fatalf("restored mode=%v", info.Mode())
	}
	if string(data) != "before" {
		t.Fatalf("undo content = %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, "new.go")); !os.IsNotExist(err) {
		t.Fatal("new file survived undo")
	}
	var hidden int
	if err := service.db.QueryRow(`SELECT count(*) FROM hidden_messages`).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if hidden != 1 {
		t.Fatalf("hidden messages = %d", hidden)
	}
	redo, err := service.Inspect(t.Context(), "session", "redo")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Apply(t.Context(), redo); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "after" {
		t.Fatalf("redo content = %q", data)
	}
	if err := service.db.QueryRow(`SELECT count(*) FROM hidden_messages`).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if hidden != 0 {
		t.Fatalf("hidden after redo = %d", hidden)
	}
}
func TestInterruptedRestoreBlocksFurtherChanges(t *testing.T) {
	t.Parallel()
	service, _ := setup(t)
	if _, err := service.db.Exec(`INSERT INTO turn_checkpoints(session_id,state,before_state) VALUES('session','applying','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Inspect(t.Context(), "session", "undo"); err == nil {
		t.Fatal("interrupted restoration ignored")
	}
}

func TestConversationDriftAndRetention(t *testing.T) {
	t.Parallel()
	service, root := setup(t)
	put(t, filepath.Join(root, "unchanged"), "keep")
	for range 12 {
		id, err := service.Begin(t.Context(), "session")
		if err != nil {
			t.Fatal(err)
		}
		if err := service.Finish(t.Context(), id, "session"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := service.db.QueryRow(`SELECT count(*) FROM turn_checkpoints`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Fatalf("retention=%d", count)
	}
	preview, err := service.Inspect(t.Context(), "session", "undo")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Before.Files) != 0 || len(preview.After.Files) != 0 {
		t.Fatal("unchanged files stored")
	}
	if _, err := service.db.Exec(`INSERT INTO messages(id,session_id) VALUES('later','session')`); err != nil {
		t.Fatal(err)
	}
	if err := service.Apply(t.Context(), preview); err == nil {
		t.Fatal("later conversation ignored")
	}
}

func TestCheckpointExcludesLiveStateWithoutGitignore(t *testing.T) {
	t.Parallel()
	service, root := setupWithStateDir(t, true)
	put(t, filepath.Join(root, "main.go"), "before")
	id, err := service.Begin(t.Context(), "session")
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "main.go"), "after")
	if err := service.Finish(t.Context(), id, "session"); err != nil {
		t.Fatal(err)
	}
	preview, err := service.Inspect(t.Context(), "session", "undo")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Paths) != 1 || preview.Paths[0] != "main.go" {
		t.Fatalf("checkpoint paths=%v", preview.Paths)
	}
	if err := service.Apply(t.Context(), preview); err != nil {
		t.Fatal(err)
	}
}
