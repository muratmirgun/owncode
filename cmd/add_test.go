package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddInstallsGloballyAndPreservesConflicts(t *testing.T) {
	home, source := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"review", "plan"} {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: Example\n---\nInstructions"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	command := newAddCmd()
	command.SetOut(&out)
	command.SetArgs([]string{source})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"review", "plan"} {
		if _, err := os.Stat(filepath.Join(home, ".owncode", "skills", name, ".owncode-install.json")); err != nil {
			t.Fatal(err)
		}
	}
	command = newAddCmd()
	command.SetArgs([]string{source})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "destination exists") {
		t.Fatalf("conflict = %v", err)
	}
	if !strings.Contains(out.String(), "Installed review") {
		t.Fatal(out.String())
	}
}

func TestAddSelectsOneSkill(t *testing.T) {
	home, source := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"one", "two"} {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: Example\n---\nInstructions"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := newAddCmd()
	command.SetArgs([]string{source, "--skill", "two"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".owncode", "skills", "one")); !os.IsNotExist(err) {
		t.Fatalf("unselected skill exists: %v", err)
	}
}
