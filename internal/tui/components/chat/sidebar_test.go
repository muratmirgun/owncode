package chat

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestSidebarShowsContextAndServices(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.Load(t.TempDir(), false)
	require.NoError(t, err)
	before := *cfg
	t.Cleanup(func() { *cfg = before })
	id := models.ModelID("sidebar-test")
	models.SupportedModels[id] = models.Model{ContextWindow: 100000}
	t.Cleanup(func() { delete(models.SupportedModels, id) })
	cfg.Agents = map[config.AgentName]config.Agent{config.AgentCoder: {Model: id}}
	cfg.MCPServers = map[string]config.MCPServer{"memory": {}}
	cfg.LSP = map[string]config.LSPConfig{"gopls": {}, "disabled": {Disabled: true}}
	m := NewSidebarCmp(session.Session{ID: "session-test", Title: "Inspect the project", PromptTokens: 12464, Cost: 0.12}, nil).(*sidebarCmp)
	m.SetSize(42, 35)
	view := ansi.Strip(m.View())
	for _, text := range []string{"Inspect the project", "session-test", "12,464 / 100,000 tokens", "12% used", "$0.12 spent", "MCP", "memory · on demand", "LSP", "gopls", "OwnCode"} {
		require.Contains(t, view, text)
	}
	require.Contains(t, view, "█")
	require.Contains(t, view, "░")
	require.NotContains(t, view, "Connected")
	require.NotContains(t, view, "disabled")
	require.Equal(t, 35, lipgloss.Height(view))
	require.Equal(t, 42, lipgloss.Width(view))
	require.Contains(t, strings.Split(view, "\n")[34], "OwnCode")
}
