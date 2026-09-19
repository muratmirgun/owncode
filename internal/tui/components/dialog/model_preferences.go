package dialog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/muratmirgun/owncode/internal/llm/models"
)

type modelPreferences struct {
	Favorites []models.ModelID `json:"favorites,omitempty"`
	Recent    []models.ModelID `json:"recent,omitempty"`
}

func modelPreferencesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "owncode", "model-preferences.json"), nil
}
func loadModelPreferences() (modelPreferences, error) {
	var prefs modelPreferences
	path, err := modelPreferencesPath()
	if err != nil {
		return prefs, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return prefs, nil
	}
	if err != nil {
		return prefs, err
	}
	err = json.Unmarshal(data, &prefs)
	return prefs, err
}
func saveModelPreferences(prefs modelPreferences) error {
	path, err := modelPreferencesPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".models-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// RecordRecentModel remembers a model after a successful selection.
func RecordRecentModel(id models.ModelID) error {
	prefs, err := loadModelPreferences()
	if err != nil {
		return fmt.Errorf("load recent models: %w", err)
	}
	prefs.Recent = slices.DeleteFunc(prefs.Recent, func(previous models.ModelID) bool { return previous == id })
	prefs.Recent = append([]models.ModelID{id}, prefs.Recent...)
	prefs.Recent = prefs.Recent[:min(6, len(prefs.Recent))]
	if err = saveModelPreferences(prefs); err != nil {
		return fmt.Errorf("save recent models: %w", err)
	}
	return nil
}
