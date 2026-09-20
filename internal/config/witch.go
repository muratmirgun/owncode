package config

import (
	"fmt"
	"slices"

	"github.com/muratmirgun/owncode/internal/llm/models"
)

// WitchModel fixes model selection for a role, outside individual dispatches.
type WitchModel struct {
	Model     models.ModelID `json:"model,omitempty"`
	Reasoning string         `json:"reasoning,omitempty"`
}

// WitchConfig stores global model preferences for the built-in Witch preset.
type WitchConfig struct {
	Lanes map[string]WitchModel `json:"lanes,omitempty"`
}

// WitchLanes returns the controller followed by the fixed worker roles.
func WitchLanes() []string {
	return []string{"controller", "witch-routine", "witch-complex", "witch-researcher", "witch-security", "witch-task-reviewer", "witch-re-reviewer", "witch-final-reviewer"}
}

// WitchLaneSettings returns the configured selection; an empty model uses the chat model.
func WitchLaneSettings(lane string) WitchModel {
	if cfg == nil {
		return WitchModel{}
	}
	return cfg.Witch.Lanes[lane]
}

// WitchAgent resolves a role without mutating the shared agent configuration.
func WitchAgent(lane string) (Agent, error) {
	if cfg == nil || !slices.Contains(WitchLanes(), lane) {
		return Agent{}, fmt.Errorf("unknown Witch role %q", lane)
	}
	selected := WitchLaneSettings(lane)
	result := cfg.Agents[AgentCoder]
	if selected.Model != "" {
		result.Model = selected.Model
		result.MaxTokens = models.SupportedModels[selected.Model].DefaultMaxTokens
	}
	if selected.Reasoning != "" {
		result.ReasoningEffort = selected.Reasoning
	}
	result.ReasoningEffort = models.SupportedModels[result.Model].ReasoningLevel(result.ReasoningEffort)
	return result, nil
}

// UpdateWitchLane persists one role while preserving unrelated global settings.
func UpdateWitchLane(lane string, selected WitchModel) error {
	if cfg == nil || !slices.Contains(WitchLanes(), lane) {
		return fmt.Errorf("unknown Witch role %q", lane)
	}
	if selected.Model != "" {
		model, ok := models.SupportedModels[selected.Model]
		if !ok {
			return fmt.Errorf("model is unavailable")
		}
		p, ok := cfg.Providers[model.Provider]
		if !ok || p.Disabled {
			return fmt.Errorf("provider is unavailable")
		}
	}
	modelID := selected.Model
	if modelID == "" {
		modelID = cfg.Agents[AgentCoder].Model
	}
	if selected.Reasoning != "" && !slices.Contains(models.SupportedModels[modelID].ReasoningChoices(), selected.Reasoning) {
		return fmt.Errorf("reasoning level is unavailable for this model")
	}
	next := WitchConfig{Lanes: make(map[string]WitchModel, len(cfg.Witch.Lanes)+1)}
	for name, value := range cfg.Witch.Lanes {
		next.Lanes[name] = value
	}
	next.Lanes[lane] = selected
	if err := saveGlobalField("witch", next); err != nil {
		return err
	}
	cfg.Witch = next
	return nil
}

// WitchControllerPrompt defines the built-in delegation and review workflow.
const WitchControllerPrompt = `You are Witch, OwnCode's primary orchestrator. You own planning, routing, verification, and final acceptance.
Use only the seven named witch-* worker roles. Models and reasoning belong to Settings > Orchestration, never dispatch overrides.
Before work, agree on the user's intent and a concrete plan. Existing user authorization counts; do not ask again for approved work.
For implementation or research, call witch_route with mutatesWorktree, securitySensitive, complexIntegration, and a concrete reason. Pass the same route fields into the agent call. The runtime checks the chosen lane.
Decompose work into bounded tasks. Give each worker exact owned paths, scope, constraints, acceptance criteria, and required checks. Parallel workers must own disjoint paths. Workers share the checkout; never revert another worker's edits.
Use witch-routine for bounded implementation, witch-complex for integration, witch-researcher for read-only evidence, and witch-security for security-sensitive changes.
Inspect results and evidence. Dispatch a fresh witch-task-reviewer. Repair findings in the original worker using worker_id, then use witch-re-reviewer. Stop after two unsuccessful repair rounds for the same finding and ask the user for direction.
After task acceptance, use a fresh witch-final-reviewer for the complete change. Accept only with FINAL VERDICT: ship. Report unresolved findings and risks. Never treat a worker's report as proof.
Research and review workers cannot edit or execute shell commands. Implementation workers use the existing approval flow. The security role cannot execute shell or network tools; run necessary verification yourself after inspection.`
