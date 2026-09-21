package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// CompactionSettings selects the summary format and automatic trigger.
type CompactionSettings struct {
	Method    string      `json:"method,omitempty"`
	Jev       JevSettings `json:"jev,omitempty"`
	Mode      string      `json:"mode,omitempty"`
	Focus     string      `json:"focus,omitempty"`
	Threshold int         `json:"threshold,omitempty"`
}

func (c CompactionSettings) EffectiveMode() string {
	switch c.Mode {
	case "brief", "handoff":
		return c.Mode
	default:
		return "balanced"
	}
}

func (c CompactionSettings) EffectiveThreshold() int {
	if c.Threshold < 50 || c.Threshold > 95 {
		return 95
	}
	return c.Threshold
}

// UpdateCompaction preserves unrelated config fields and updates memory after saving.
func UpdateCompaction(settings CompactionSettings, automatic bool) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	path := viper.ConfigFileUsed()
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".owncode.json")
	}
	if err := saveCompaction(path, settings, automatic); err != nil {
		return err
	}
	cfg.Compaction = settings
	cfg.AutoCompact = automatic
	return nil
}

func saveCompaction(path string, settings CompactionSettings, automatic bool) error {
	fields := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields["compaction"], err = json.Marshal(settings)
	if err != nil {
		return err
	}
	fields["autoCompact"], err = json.Marshal(automatic)
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
	return os.Rename(file.Name(), path)
}

// JevSettings configures the Compact Engine scoring client.
type JevSettings struct {
	SummaryFallback bool   `json:"summaryFallback,omitempty"`
	APIKey          string `json:"apiKey,omitempty"`
	Model           string `json:"model,omitempty"`
	TargetTokens    int    `json:"targetTokens,omitempty"`
}

// EffectiveMethod defaults to the existing summary behavior.
func (c CompactionSettings) EffectiveMethod() string {
	if c.Method == "" {
		return "summary"
	}
	return c.Method
}
