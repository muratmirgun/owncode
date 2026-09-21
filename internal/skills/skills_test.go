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

func TestGlobalSkillLinksAndMentions(t *testing.T) {
	home, workdir, source := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	target := filepath.Dir(fixture(t, source, "review", "linked instructions"))
	shared := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(shared, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(shared, "review")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	entries, problems := Discover(t.Context(), Roots(workdir))
	if len(entries) != 1 || len(problems) != 0 {
		t.Fatalf("discovery: %v %v", entries, problems)
	}
	content, err := ExpandInvocation(t.Context(), workdir, "Please use @skill/review on this code")
	if err != nil || !strings.Contains(content, "linked instructions") {
		t.Fatalf("mention: %s %v", content, err)
	}
	if err := os.Symlink(filepath.Join(home, "secret"), filepath.Join(target, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(entries[0], "escape"); err == nil {
		t.Fatal("supporting file escaped")
	}
	other := filepath.Dir(fixture(t, t.TempDir(), "review", "replacement"))
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(entries[0], ""); err == nil {
		t.Fatal("retargeted link loaded")
	}
	fixture(t, filepath.Join(home, ".owncode", "skills"), "review", "owncode instructions")
	content, err = ExpandInvocation(t.Context(), workdir, "@skill/review test")
	if err != nil || !strings.Contains(content, "owncode instructions") {
		t.Fatalf("precedence: %s %v", content, err)
	}
}

func TestProjectLinksRemainConfined(t *testing.T) {
	root, source := t.TempDir(), t.TempDir()
	target := filepath.Dir(fixture(t, source, "review", "external"))
	if err := os.Symlink(target, filepath.Join(root, "review")); err != nil {
		t.Fatal(err)
	}
	entries, _ := Discover(t.Context(), []Root{{root, "project"}})
	if len(entries) != 0 {
		t.Fatal("external project link accepted")
	}
}

func TestPrepareAllSelectionAndDuplicates(t *testing.T) {
	source := t.TempDir()
	fixture(t, source, "one", "first")
	fixture(t, source, "two", "second")
	proposals, err := PrepareAll(t.Context(), source, "")
	if err != nil || len(proposals) != 2 {
		t.Fatalf("all: %v %v", proposals, err)
	}
	proposals, err = PrepareAll(t.Context(), source, "two")
	if err != nil || len(proposals) != 1 || proposals[0].Entry.Name != "two" {
		t.Fatalf("selection: %v %v", proposals, err)
	}
	fixture(t, filepath.Join(source, "nested"), "one", "duplicate")
	if _, err := PrepareAll(t.Context(), source, ""); err == nil {
		t.Fatal("duplicate names accepted")
	}
	proposals, err = PrepareAll(t.Context(), source+"#two", "")
	if err != nil || len(proposals) != 1 {
		t.Fatalf("subdirectory: %v %v", proposals, err)
	}
}

func TestRepositorySourcesFetchOnce(t *testing.T) {
	source, bin := t.TempDir(), t.TempDir()
	fixture(t, source, "review", "remote skill")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The fake Git executable asserts URL normalization and supplies a local checkout.
	script := "#!/bin/sh\ncase \"$*\" in\n*clone*)\n previous=\"\"\n for arg do previous2=\"$previous\"; previous=\"$arg\"; done\n [ \"$previous2\" = \"https://github.com/example/skills\" ] || exit 9\n cp -R \"" + source + "\" \"$previous\"\n ;;\n*rev-parse*) echo abc123 ;;\n*) exit 8 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"@example/skills", "https://github.com/example/skills"} {
		proposals, err := PrepareAll(t.Context(), source, "")
		if err != nil || len(proposals) != 1 {
			t.Fatalf("%s: %v %v", source, proposals, err)
		}
		manifest := proposals[0].Manifest
		if manifest.Source != "https://github.com/example/skills" || manifest.Revision != "abc123" {
			t.Fatalf("manifest: %#v", manifest)
		}
	}
}
