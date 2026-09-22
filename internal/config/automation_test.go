package config

import (
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalConfigCannotEnableAutomation(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".owncode.json"), []byte(`{"automation":{"browser":"extension","computer":"macos"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	viper.Set("automation.browser", "off")
	if err := mergeLocalConfig(dir); err != nil {
		t.Fatal(err)
	}
	if got := viper.GetString("automation.computer"); got != "" {
		t.Fatalf("local config enabled computer: %s", got)
	}
}
