package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveCompactionPreservesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".owncode.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"provider":{"custom":{"options":{"apiKey":"test-secret"}}},"futureSetting":{"enabled":true}}`), 0600))
	settings := CompactionSettings{Method: "jev", Jev: JevSettings{APIKey: "fake-jev-key", TargetTokens: 10000}, Mode: "handoff", Focus: "Keep test failures", Threshold: 80}
	require.NoError(t, saveCompaction(path, settings, false))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var values map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &values))
	require.JSONEq(t, `{"custom":{"options":{"apiKey":"test-secret"}}}`, string(values["provider"]))
	require.JSONEq(t, `{"enabled":true}`, string(values["futureSetting"]))
	require.Equal(t, "false", string(values["autoCompact"]))
	var decoded Config
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, settings, decoded.Compaction)
	stat, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), stat.Mode().Perm())
}

func TestCompactionDefaults(t *testing.T) {
	settings := CompactionSettings{Mode: "unknown", Threshold: 101}
	require.Equal(t, "balanced", settings.EffectiveMode())
	require.Equal(t, 95, settings.EffectiveThreshold())
}
