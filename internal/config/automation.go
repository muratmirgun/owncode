package config

import (
	"fmt"
	"slices"
	"sync/atomic"
)

// AutomationSettings selects optional device adapters. Empty backends disable access.
type AutomationSettings struct {
	Browser    string `json:"browser,omitempty"`
	Computer   string `json:"computer,omitempty"`
	Executable string `json:"executable,omitempty"`
	CDPURL     string `json:"cdpURL,omitempty"`
}

var automationSnapshot atomic.Value

// SetAutomationSnapshot publishes an immutable copy for tool goroutines.
func SetAutomationSnapshot(settings AutomationSettings) { automationSnapshot.Store(settings) }

// CurrentAutomation returns the device settings without reading mutable UI state.
func CurrentAutomation() AutomationSettings {
	if value := automationSnapshot.Load(); value != nil {
		return value.(AutomationSettings)
	}
	return AutomationSettings{}
}

// UpdateAutomation saves global preferences. Existing sessions keep their backend until closed.
func UpdateAutomation(settings AutomationSettings) error {
	if cfg == nil {
		return fmt.Errorf("configuration is not loaded")
	}
	if !slices.Contains([]string{"", "off", "embedded", "chrome", "brave", "cdp", "extension"}, settings.Browser) {
		return fmt.Errorf("unknown browser backend %q", settings.Browser)
	}
	if !slices.Contains([]string{"", "off", "macos"}, settings.Computer) {
		return fmt.Errorf("unknown computer backend %q", settings.Computer)
	}
	if err := saveGlobalField("automation", settings); err != nil {
		return err
	}
	cfg.Automation = settings
	SetAutomationSnapshot(settings)
	return nil
}
