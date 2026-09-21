package completions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillCompletionUsesGlobalLink(t *testing.T) {
	home, source := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: review\ndescription: Check changes\n---\nRead the diff."), 0600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".agent", "skills")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(root, "review")); err != nil {
		t.Fatal(err)
	}
	cg := NewFileAndFolderContextGroup()
	_, _ = cg.GetChildEntries("")
	entries, err := cg.GetChildEntries("skill/rev")
	if err != nil || len(entries) != 1 || entries[0].GetValue() != "@skill/review " {
		t.Fatalf("completion: %v %v", entries, err)
	}
}
