package config

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/muratmirgun/owncode/internal/auth"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConnectionRegistrationAndSelection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	previous := cfg
	catalog := maps.Clone(models.SupportedModels)
	t.Cleanup(func() { cfg = previous; models.SupportedModels = catalog; viper.Reset() })
	cfg = &Config{Providers: map[models.ModelProvider]Provider{}, Agents: map[AgentName]Agent{}}
	connection := auth.Connection{Token: &auth.Token{Access: "secret-access", Refresh: "secret-refresh", Expires: time.Now().Add(time.Hour)}, Models: []auth.Model{{ID: "model", Name: "Test Model", Context: 200000, Output: 8192}}}
	require.NoError(t, RegisterConnection(auth.ChatGPT, connection))
	require.True(t, cfg.Providers[auth.ChatGPT].HasCredentials())
	require.Empty(t, cfg.Providers[auth.ChatGPT].APIKey)
	path := filepath.Join(dir, "project.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"future":{"keep":true},"compaction":{"method":"shake"}}`), 0600))
	viper.SetConfigFile(path)
	require.NoError(t, viper.ReadInConfig())
	require.NoError(t, SelectConnectedModel("chatgpt/model"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(data), "secret-")
	var stored map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &stored))
	require.JSONEq(t, `{"keep":true}`, string(stored["future"]))
	for _, role := range []AgentName{AgentCoder, AgentTitle, AgentTask, AgentSummarizer} {
		require.Equal(t, models.ModelID("chatgpt/model"), cfg.Agents[role].Model)
	}
	loaded := &Config{Providers: map[models.ModelProvider]Provider{}}
	require.NoError(t, loadConnections(loaded))
	require.Equal(t, auth.ChatGPT, loaded.Providers[auth.ChatGPT].Auth)
	require.Equal(t, "Test Model", loaded.Providers[auth.ChatGPT].Models["model"].Name)
	m := models.SupportedModels["chatgpt/model"]
	m.ReasoningLevels = []string{"low", "medium", "high", "xhigh"}
	m.DefaultReasoning = "medium"
	models.SupportedModels[m.ID] = m
	level, err := CycleReasoning()
	require.NoError(t, err)
	require.Equal(t, "high", level)
	level, err = CycleReasoning()
	require.NoError(t, err)
	require.Equal(t, "xhigh", level)
	require.NoError(t, validateAgent(cfg, AgentCoder, cfg.Agents[AgentCoder]))
	require.Equal(t, "xhigh", cfg.Agents[AgentCoder].ReasoningEffort)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	var persisted Config
	require.NoError(t, json.Unmarshal(data, &persisted))
	require.Equal(t, "xhigh", persisted.Agents[AgentCoder].ReasoningEffort)
	require.Equal(t, "", persisted.Agents[AgentTitle].ReasoningEffort)
}
