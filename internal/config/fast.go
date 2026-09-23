package config

import (
	"sync/atomic"

	"github.com/muratmirgun/owncode/internal/llm/models"
)

var fastMode atomic.Bool

// FastMode reports the process-local priority preference. It resets on restart.
func FastMode() bool { return fastMode.Load() }

// SetFastMode changes the tier for subsequent supported requests, including workers.
func SetFastMode(enabled bool) { fastMode.Store(enabled) }

// SupportsFast identifies the provider adapters with priority request support.
// The provider still decides whether the account and model can use that tier.
func SupportsFast(model models.Model) bool {
	return model.Provider == "chatgpt" || (model.Provider == models.ProviderOpenAI && !model.Custom)
}
