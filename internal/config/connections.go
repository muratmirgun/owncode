package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/muratmirgun/owncode/internal/auth"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/spf13/viper"
)

// HasCredentials includes managed ChatGPT login without placing tokens in config.
func (p Provider) HasCredentials() bool { return p.APIKey != "" || p.Auth == auth.ChatGPT }

func connectionProvider(id string, c auth.Connection) Provider {
	p := Provider{APIKey: c.Key, BaseURL: "https://api.anthropic.com", Models: map[string]CustomModel{}}
	if id == auth.ChatGPT {
		p.APIKey = ""
		p.Auth = auth.ChatGPT
		p.BaseURL = auth.CodexURL
	}
	for _, m := range c.Models {
		options := map[string]any{}
		if id == auth.ChatGPT {
			options["native_compaction"] = true
		}
		p.Models[m.ID] = CustomModel{Name: m.Name, ContextWindow: m.Context, MaxTokens: m.Output, Attachments: true, Options: options, ReasoningLevels: m.ReasoningLevels, DefaultReasoning: m.DefaultReasoning, Reasoning: len(m.ReasoningLevels) > 0}
	}
	return p
}

func loadConnections(c *Config) error {
	connections, err := auth.Connections()
	if err != nil {
		return err
	}
	for _, id := range []string{auth.ChatGPT, auth.Claude} {
		connection, ok := connections[id]
		if !ok {
			continue
		}
		if id == auth.ChatGPT && connection.Token == nil {
			continue
		}
		c.Providers[models.ModelProvider(id)] = connectionProvider(id, connection)
	}
	return nil
}

// RegisterConnection saves a validated connection and updates the idle app's catalog.
func RegisterConnection(id string, c auth.Connection) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if id != auth.ChatGPT && id != auth.Claude {
		return fmt.Errorf("unsupported provider")
	}
	if len(c.Models) == 0 || (id == auth.ChatGPT && c.Token == nil) || (id == auth.Claude && c.Key == "") {
		return fmt.Errorf("connection has no credentials or models")
	}
	for _, model := range c.Models {
		if model.ID == "" || model.Name == "" || model.Context <= 0 || model.Output <= 0 || model.Output > model.Context {
			return fmt.Errorf("connection contains an invalid model")
		}
	}
	if err := auth.Save(id, c); err != nil {
		return err
	}
	p := connectionProvider(id, c)
	cfg.Providers[models.ModelProvider(id)] = p
	for _, m := range c.Models {
		modelID := models.ModelID(id + "/" + m.ID)
		models.SupportedModels[modelID] = models.Model{ID: modelID, APIModel: m.ID, Name: m.Name, Provider: models.ModelProvider(id), Custom: true,
			ContextWindow: m.Context, DefaultMaxTokens: m.Output, SupportsAttachments: true, Options: p.Models[m.ID].Options, CanReason: len(m.ReasoningLevels) > 0, ReasoningLevels: m.ReasoningLevels, DefaultReasoning: m.DefaultReasoning}
	}
	return nil
}

// SelectConnectedModel changes all agent roles together and preserves unrelated config fields.
func SelectConnectedModel(id models.ModelID) error {
	model, ok := models.SupportedModels[id]
	if !ok {
		return fmt.Errorf("unknown connection model")
	}
	next := maps.Clone(cfg.Agents)
	if next == nil {
		next = map[AgentName]Agent{}
	}
	for _, role := range []AgentName{AgentCoder, AgentTitle, AgentTask, AgentSummarizer} {
		next[role] = Agent{Model: id, MaxTokens: model.DefaultMaxTokens, ReasoningEffort: model.ReasoningLevel("")}
	}
	return saveAgents(next)
}

func saveAgents(next map[AgentName]Agent) error {
	path := viper.ConfigFileUsed()
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".owncode.json")
	}
	fields := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 && json.Unmarshal(data, &fields) != nil {
		return fmt.Errorf("invalid config file")
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields["agents"], err = json.Marshal(next)
	if err != nil {
		return err
	}
	data, err = json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".owncode-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	viper.SetConfigFile(path)
	cfg.Agents = next
	return nil
}

// CycleReasoning saves the next supported level for the coding agent.
func CycleReasoning() (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}
	settings := cfg.Agents[AgentCoder]
	model := models.SupportedModels[settings.Model]
	choices := model.ReasoningChoices()
	if len(choices) == 0 {
		return "", fmt.Errorf("this model has no configurable reasoning levels; reconnect ChatGPT to refresh its model catalog")
	}
	current := model.ReasoningLevel(settings.ReasoningEffort)
	index := 0
	for i, level := range choices {
		if level == current {
			index = (i + 1) % len(choices)
			break
		}
	}
	settings.ReasoningEffort = choices[index]
	next := maps.Clone(cfg.Agents)
	next[AgentCoder] = settings
	if err := saveAgents(next); err != nil {
		return "", err
	}
	return settings.ReasoningEffort, nil
}
