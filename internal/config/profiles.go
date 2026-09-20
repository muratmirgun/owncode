package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/viper"

	"github.com/muratmirgun/owncode/internal/llm/models"
)

// Profile configures a primary agent without changing the auxiliary model roles.
type Profile struct {
	Description string         `json:"description,omitempty"`
	Model       models.ModelID `json:"model,omitempty"`
	Reasoning   string         `json:"reasoning,omitempty"`
	Prompt      string         `json:"prompt,omitempty"`
	ReadOnly    bool           `json:"readOnly,omitempty"`
	Tools       []string       `json:"tools,omitempty"`
}

// ProfileNames returns built-in profiles followed by configured profiles.
func ProfileNames() []string {
	names := []string{"build", "plan"}
	if cfg == nil {
		return names
	}
	extra := []string{}
	for name := range cfg.Profiles {
		if name != "build" && name != "plan" {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	return append(names, extra...)
}

// CurrentProfile returns the active profile and its resolved defaults.
func CurrentProfile() (string, Profile) {
	if cfg == nil {
		return "build", Profile{}
	}
	name := cfg.ActiveProfile
	if name == "" {
		name = "build"
	}
	profile, known := cfg.Profiles[name]
	if !known && name != "build" && name != "plan" {
		return "plan", Profile{ReadOnly: true, Prompt: "The configured profile is unknown. Explain the issue. Do not modify files."}
	}
	if name == "plan" {
		profile.ReadOnly = true
		if profile.Prompt == "" {
			profile.Prompt = "Analyze and plan. Do not change files. Return a concrete plan with verification steps."
		}
	}
	return name, profile
}

// SelectProfile persists a profile choice after validating its model.
func SelectProfile(name string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	found := false
	for _, candidate := range ProfileNames() {
		if candidate == name {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("unknown profile %q", name)
	}
	profile := cfg.Profiles[name]
	if profile.Model != "" {
		if _, ok := models.SupportedModels[profile.Model]; !ok {
			return fmt.Errorf("profile model is unavailable")
		}
	}
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
	if len(data) > 0 {
		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields["activeProfile"], _ = json.Marshal(name)
	data, err = json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".owncode-profile-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }() // Best-effort cleanup; preserve the primary result.
	if _, err = f.Write(data); err != nil {
		_ = f.Close() // The read/transaction result determines success.
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	cfg.ActiveProfile = name
	return nil
}
