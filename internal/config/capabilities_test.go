package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/muratmirgun/owncode/internal/llm/models"
)

func TestCompactionCapabilities(t *testing.T) {
	previous := cfg
	t.Cleanup(func() { cfg = previous })
	cfg = &Config{Agents: map[AgentName]Agent{AgentCoder: {Model: "capability-test"}}}
	models.SupportedModels["capability-test"] = models.Model{ID: "capability-test", Provider: "compatible"}
	t.Cleanup(func() { delete(models.SupportedModels, "capability-test") })
	if CompactionUnavailable("native") == "" {
		t.Fatal("unsupported native method allowed")
	}
	if CompactionUnavailable("snapcompact") == "" {
		t.Fatal("text model accepted")
	}
	if CompactionUnavailable("jev") == "" {
		t.Fatal("missing Jev key ignored")
	}
	if CompactionUnavailable("summary") != "" {
		t.Fatal("summary rejected")
	}
	cfg.ActiveProfile = "typo"
	_, profile := CurrentProfile()
	if !profile.ReadOnly {
		t.Fatal("unknown profile allowed writes")
	}
}
func TestProjectCannotEnableExtension(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".owncode.json"), []byte(`{"extensions":{"bad":{"enabled":true,"command":["/tmp/bad"]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mergeLocalConfig(dir); err != nil {
		t.Fatal(err)
	}
	if len(viper.GetStringMap("extensions")) != 0 {
		t.Fatal("project hook enabled")
	}
}
