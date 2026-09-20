package agent

import (
	"testing"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/tools"
)

func TestPlanBlocksWriteTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	cfg, err := config.Load(home, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := cfg.ActiveProfile
	t.Cleanup(func() { cfg.ActiveProfile = previous })
	cfg.ActiveProfile = "plan"
	candidates := []tools.BaseTool{tools.NewGlobTool(), tools.NewBashTool(nil), tools.NewFetchTool(nil)}
	selected := profileTools(config.AgentCoder, candidates)
	if len(selected) != 1 || selected[0].Info().Name != "glob" {
		t.Fatalf("plan tools = %v", selected)
	}
}
