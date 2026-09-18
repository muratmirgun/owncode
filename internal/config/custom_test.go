package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestCustomProviderLocalConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".owncode.json"), []byte(`{"lsp":{"gopls":{"command":"gopls"}}}`), 0600))
	local := `{"providers":{"test-custom":{"baseURL":"https://example.com/v1","apiKey":"private-test-key","models":{"qwen":{"name":"Qwen","contextWindow":262144,"maxTokens":32768,"reasoning":true,"attachments":true,"interleaved":"reasoning_content","options":{"top_p":0.95,"chat_template_kwargs":{"enable_thinking":true}}}}}},"agents":{"coder":{"model":"test-custom/qwen"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".owncode.local.json"), []byte(local), 0600))
	require.NoError(t, mergeLocalConfig(dir))
	var c Config
	require.NoError(t, viper.Unmarshal(&c))
	require.Equal(t, "gopls", c.LSP["gopls"].Command)
	require.Equal(t, "private-test-key", c.Providers["test-custom"].APIKey)
	require.NoError(t, registerCustomModels(&c))
	t.Cleanup(func() { delete(models.SupportedModels, "test-custom/qwen") })
	model := models.SupportedModels["test-custom/qwen"]
	require.Equal(t, int64(262144), model.ContextWindow)
	require.Equal(t, int64(32768), model.DefaultMaxTokens)
	require.Equal(t, "reasoning_content", model.ReasoningField)
	require.Equal(t, true, model.Options["chat_template_kwargs"].(map[string]any)["enable_thinking"])
	require.True(t, model.SupportsAttachments)
	require.True(t, model.Custom)
}

func TestMalformedLocalConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".owncode.local.json"), []byte(`{broken`), 0600))
	require.ErrorContains(t, mergeLocalConfig(dir), ".owncode.local.json")
}

func TestCustomProviderValidation(t *testing.T) {
	for _, endpoint := range []string{"", "file:///tmp/model", "https://"} {
		t.Run(endpoint, func(t *testing.T) {
			c := &Config{Providers: map[models.ModelProvider]Provider{
				"custom-invalid": {BaseURL: endpoint, Models: map[string]CustomModel{
					"qwen": {Name: "Qwen", ContextWindow: 262144, MaxTokens: 32768},
				}},
			}}
			require.ErrorContains(t, registerCustomModels(c), "baseURL")
			require.NotContains(t, models.SupportedModels, models.ModelID("custom-invalid/qwen"))
		})
	}
}
