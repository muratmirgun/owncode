package tui

import (
	tea "charm.land/bubbletea/v2"
	"maps"
	"testing"
	"time"

	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/auth"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
	"github.com/muratmirgun/owncode/internal/tui/page"
	"github.com/stretchr/testify/require"
)

func TestConnectActivatesAnUnconfiguredAgent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	cfg, err := config.Load(dir, false)
	require.NoError(t, err)
	old := *cfg
	catalog := maps.Clone(models.SupportedModels)
	t.Cleanup(func() { *cfg = old; models.SupportedModels = catalog })
	cfg.Agents = map[config.AgentName]config.Agent{}
	cfg.Providers = map[models.ModelProvider]config.Provider{}
	svc, err := agent.NewAgent(config.AgentCoder, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, svc.Model().ID)
	ui := appModel{app: &app.App{CoderAgent: svc}, showConnect: true, connect: dialog.NewConnectCmp()}
	m := auth.Model{ID: "test-model", Name: "Test Model", Context: 200000, Output: 8192, ReasoningLevels: []string{"low", "medium", "high", "xhigh"}, DefaultReasoning: "medium"}
	updated, _ := ui.Update(dialog.ProviderConnectedMsg{ID: auth.ChatGPT, Model: m, Connection: auth.Connection{Token: &auth.Token{Access: "fake", Expires: time.Now().Add(time.Hour)}, Models: []auth.Model{m}}})
	require.False(t, updated.(appModel).showConnect)
	require.Equal(t, models.ModelID("chatgpt/test-model"), svc.Model().ID)
	for _, role := range []config.AgentName{config.AgentTitle, config.AgentSummarizer, config.AgentTask} {
		require.Equal(t, svc.Model().ID, cfg.Agents[role].Model)
	}
	active := updated.(appModel)
	active.currentPage = page.ChatPage
	_, cmd := active.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModAlt})
	require.NotNil(t, cmd)
	require.Equal(t, "high", cfg.Agents[config.AgentCoder].ReasoningEffort)
	require.Equal(t, "medium", cfg.Agents[config.AgentTitle].ReasoningEffort)
}

func TestModelPickerOwnsShortcuts(t *testing.T) {
	ui := appModel{showModelDialog: true, modelDialog: dialog.NewModelDialogCmp()}
	updated, cmd := ui.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	require.False(t, updated.(appModel).showFilepicker)
	require.Nil(t, cmd)
	_, cmd = ui.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	require.IsType(t, dialog.ConnectModelProviderMsg{}, cmd())
	updated, cmd = ui.Update(dialog.ConnectModelProviderMsg{})
	require.False(t, updated.(appModel).showModelDialog)
	require.NotNil(t, cmd)
	_, cmd = ui.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	require.IsType(t, dialog.CloseModelDialogMsg{}, cmd())
}

func TestCatalogRefreshKeepsSelectionAndUpdatesPicker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	cfg, err := config.Load(dir, false)
	require.NoError(t, err)
	old := *cfg
	catalog := maps.Clone(models.SupportedModels)
	t.Cleanup(func() { *cfg = old; models.SupportedModels = catalog })
	cfg.Agents = map[config.AgentName]config.Agent{}
	cfg.Providers = map[models.ModelProvider]config.Provider{}
	connection := auth.Connection{Token: &auth.Token{Access: "fake", Expires: time.Now().Add(time.Hour)}, Models: []auth.Model{
		{ID: "old", Name: "Old Model", Context: 200000, Output: 8192},
	}}
	require.NoError(t, config.RegisterConnection(auth.ChatGPT, connection))
	for _, role := range []config.AgentName{config.AgentCoder, config.AgentTitle, config.AgentTask, config.AgentSummarizer} {
		cfg.Agents[role] = config.Agent{Model: "chatgpt/old", MaxTokens: 8192}
	}
	svc, err := agent.NewAgent(config.AgentCoder, nil, nil, nil)
	require.NoError(t, err)
	picker := dialog.NewModelDialogCmp()
	picker.Init()
	ui := appModel{app: &app.App{CoderAgent: svc}, modelDialog: picker, showModelDialog: true}
	connection.Models = []auth.Model{{ID: "new", Name: "New Model", Context: 200000, Output: 8192}}
	updated, _ := ui.Update(modelCatalogResultMsg{provider: auth.ChatGPT, connection: connection})
	current := updated.(appModel)
	require.Equal(t, models.ModelID("chatgpt/old"), cfg.Agents[config.AgentCoder].Model)
	require.Equal(t, models.ModelID("chatgpt/old"), svc.Model().ID)
	require.Contains(t, current.modelDialog.View(), "New Model")
	require.NotContains(t, current.modelDialog.View(), "Old Model")
	saved, err := auth.Connections()
	require.NoError(t, err)
	require.Equal(t, "new", saved[auth.ChatGPT].Models[0].ID)
}
