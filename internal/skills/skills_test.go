package skills

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: "+name+"\ndescription: Test workflow\n---\n"+body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestDiscoveryPrecedenceAndDisabled(t *testing.T) {
	t.Parallel()
	project, user := t.TempDir(), t.TempDir()
	fixture(t, project, "review", "project instructions")
	fixture(t, user, "review", "user instructions")
	entries, problems := Discover(t.Context(), []Root{{project, "project"}, {user, "user"}})
	if len(problems) != 0 || len(entries) != 1 || len(entries[0].Shadowed) != 1 {
		t.Fatalf("Discover = %v, %v", entries, problems)
	}
	loaded, err := Load(entries[0], "")
	if err != nil || !strings.Contains(loaded, "project instructions") {
		t.Fatalf("Load = %q, %v", loaded, err)
	}
	if err := SetEnabled(entries[0], false); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(entries[0], ""); err == nil {
		t.Fatal("disabled skill loaded")
	}
	entries, _ = Discover(t.Context(), []Root{{project, "project"}, {user, "user"}})
	if !entries[0].Disabled {
		t.Fatal("disabled project skill fell through to user")
	}
}
func TestReadConfinementAndRevision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := fixture(t, root, "review", "instructions")
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(filepath.Dir(path), "escape")); err != nil {
		t.Fatal(err)
	}
	entries, _ := Discover(t.Context(), []Root{{root, "project"}})
	for _, name := range []string{"../secret", "escape"} {
		if _, err := Load(entries[0], name); err == nil {
			t.Fatalf("Load(%q) escaped root", name)
		}
	}
	fixture(t, root, "review", "new instructions")
	if _, err := Load(entries[0], ""); err == nil {
		t.Fatal("changed skill loaded without refresh")
	}
}
func TestParseInvalid(t *testing.T) {
	t.Parallel()
	for _, data := range []string{"no frontmatter", "---\nname: bad\n", "---\nname: ../bad\ndescription: test\n---\nbody", strings.Repeat("a", MaxContent+1)} {
		if _, _, err := Parse([]byte(data)); err == nil {
			t.Fatal("invalid skill accepted")
		}
	}
}
func TestInstallUpdateAndLocalEdits(t *testing.T) {
	t.Parallel()
	source, destination := t.TempDir(), t.TempDir()
	fixture(t, source, "review", "first")
	proposal, err := Prepare(t.Context(), source, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(destination, proposal, ""); err != nil {
		t.Fatal(err)
	}
	entries, _ := Discover(t.Context(), []Root{{destination, "project"}})
	if len(entries) != 1 {
		t.Fatalf("installed catalog = %v", entries)
	}
	if err := Apply(destination, proposal, ""); err == nil {
		t.Fatal("existing skill overwritten")
	}
	fixture(t, source, "review", "second")
	next, err := Prepare(t.Context(), source, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(destination, next, proposal.Manifest.Digest); err != nil {
		t.Fatal(err)
	}
	fixture(t, destination, "review", "user edit")
	if err := Apply(destination, proposal, next.Manifest.Digest); err == nil {
		t.Fatal("local edit overwritten")
	}
	entries, _ = Discover(t.Context(), []Root{{destination, "project"}})
	if err := Remove(entries[0]); err == nil {
		t.Fatal("local edit removed")
	}
}
func TestPrepareRejectsLinksAndCancellation(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	path := fixture(t, source, "review", "instructions")
	if err := os.Symlink("SKILL.md", filepath.Join(filepath.Dir(path), "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(t.Context(), source, ""); err == nil {
		t.Fatal("install accepted symlink")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Prepare(ctx, source, ""); err == nil {
		t.Fatal("canceled prepare succeeded")
	}
}
func TestCatalog(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/skills/search" || r.URL.Query().Get("q") != "go review" {
			t.Errorf("request = %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer test" {
			w.WriteHeader(401)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[{"name":"Review","slug":"review","source":"owner/repo"}]}`)
	}))
	defer server.Close()
	results, err := Search(t.Context(), server.URL, "test", "go review")
	if err != nil || len(results) != 1 {
		t.Fatalf("Search = %v, %v", results, err)
	}
	if _, err := Search(t.Context(), server.URL, "", "go review"); err == nil {
		t.Fatal("unauthorized catalog accepted")
	}
}

func TestRollbackAndExplicitInvocation(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	fixture(t, source, "review", "first")
	original, err := Prepare(t.Context(), source, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(destination, original, ""); err != nil {
		t.Fatal(err)
	}
	fixture(t, source, "review", "second")
	next, err := Prepare(t.Context(), source, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(destination, next, original.Manifest.Digest); err != nil {
		t.Fatal(err)
	}
	entries, _ := Discover(t.Context(), []Root{{destination, "project"}})
	rollback, err := RollbackProposal(t.Context(), entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if rollback.Manifest.Digest != original.Manifest.Digest {
		t.Fatal("wrong rollback")
	}
	if err := Apply(destination, rollback, next.Manifest.Digest); err != nil {
		t.Fatal(err)
	}
	workdir := t.TempDir()
	fixture(t, filepath.Join(workdir, ".owncode", "skills"), "review", "Use this review skill")
	content, err := ExpandInvocation(t.Context(), workdir, "$review inspect this code")
	if err != nil || !strings.Contains(content, "Use this review skill") {
		t.Fatalf("%s %v", content, err)
	}
}
