package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserSetupExtractsBundledExtension(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	command := newBrowserCmd()
	command.SetArgs([]string{"setup"})
	var out bytes.Buffer
	command.SetOut(&out)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".owncode", "browser-extension", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version int `json:"manifest_version"`
	}
	if err = json.Unmarshal(data, &manifest); err != nil || manifest.Version != 3 {
		t.Fatal("invalid extension manifest")
	}
	for _, name := range []string{"popup.html", "popup.js", "background.js"} {
		if _, err = os.Stat(filepath.Join(filepath.Dir(path), name)); err != nil {
			t.Fatal(err)
		}
	}
}
