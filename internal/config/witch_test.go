package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestWitchModelsAreIndependentAndPersisted(t *testing.T) {
	old := cfg
	t.Cleanup(func() {
		cfg = old
		viper.Reset()
		delete(models.SupportedModels, "witch-test-a")
		delete(models.SupportedModels, "witch-test-b")
	})
	viper.Reset()
	dir := t.TempDir()
	file := filepath.Join(dir, ".owncode.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"unrelated":{"keep":true}}`), 0600))
	viper.SetConfigFile(file)
	models.SupportedModels["witch-test-a"] = models.Model{ID: "witch-test-a", Provider: "test", ReasoningLevels: []string{"low", "high"}}
	models.SupportedModels["witch-test-b"] = models.Model{ID: "witch-test-b", Provider: "test", ReasoningLevels: []string{"medium", "max"}}
	cfg = &Config{Agents: map[AgentName]Agent{AgentCoder: {Model: "witch-test-a", ReasoningEffort: "high"}}, Providers: map[models.ModelProvider]Provider{"test": {}}}
	require.NoError(t, UpdateWitchLane("witch-routine", WitchModel{Model: "witch-test-b", Reasoning: "max"}))
	routine, err := WitchAgent("witch-routine")
	require.NoError(t, err)
	require.Equal(t, models.ModelID("witch-test-b"), routine.Model)
	require.Equal(t, "max", routine.ReasoningEffort)
	controller, err := WitchAgent("controller")
	require.NoError(t, err)
	require.Equal(t, models.ModelID("witch-test-a"), controller.Model)
	require.Error(t, UpdateWitchLane("witch-routine", WitchModel{Model: "witch-test-b", Reasoning: "high"}))
	require.Error(t, UpdateWitchLane("arbitrary", WitchModel{}))
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	var saved map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &saved))
	require.Contains(t, string(saved["unrelated"]), "true")
	require.Contains(t, string(saved["witch"]), "max")
	cfg.ActiveProfile = "witch"
	require.Equal(t, models.ModelID("witch-test-a"), EffectiveCoder().Model)
	require.NoError(t, UpdateWitchLane("controller", WitchModel{Model: "witch-test-b", Reasoning: "medium"}))
	require.Equal(t, models.ModelID("witch-test-b"), EffectiveCoder().Model)
	require.Equal(t, "medium", EffectiveCoder().ReasoningEffort)
	require.Contains(t, ProfileNames(), "witch")
	require.NoError(t, UpdateWitchLane("witch-routine", WitchModel{}))
	routine, err = WitchAgent("witch-routine")
	require.NoError(t, err)
	require.Equal(t, models.ModelID("witch-test-a"), routine.Model)
}
