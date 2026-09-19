package agent

import (
	"fmt"
	"strings"

	"github.com/muratmirgun/owncode/internal/message"
)

// CompactOptions controls the summary format and information to retain.
type CompactOptions struct {
	Method string
	Mode   string
	Focus  string
}

func (o CompactOptions) validate() error {
	switch o.Method {
	case "", "summary", "shake", "snapcompact", "jev", "native":
	default:
		return fmt.Errorf("unknown compact method: %s", o.Method)
	}
	switch o.Mode {
	case "", "balanced", "brief", "handoff":
		return nil
	default:
		return fmt.Errorf("unknown compact mode: %s", o.Mode)
	}
}

func (o CompactOptions) prompt() string {
	prompt := "Summarize this conversation for continued coding. Preserve the user's goal, constraints, decisions, changed files, verification results, unresolved errors, and next steps. Do not invent completed work."
	switch o.Mode {
	case "brief":
		prompt += " Be brief: aim for fewer than 500 words. Keep essential paths, commands, and unresolved blockers."
	case "handoff":
		prompt += " Write a structured handoff with headings: Goal, Current state, Decisions, Files, Verification, Risks, Next steps. Include exact commands and facts another agent needs to resume."
	default:
		prompt += " Balance detail and brevity. Preserve technical context needed to continue."
	}
	if focus := strings.TrimSpace(o.Focus); focus != "" {
		prompt += "\nUser focus for this compaction:\n" + focus
	}
	return prompt
}

func activeSummaryMessages(messages []message.Message, summaryID string) []message.Message {
	// A snapshot is active only after the session pointer commits successfully.
	active := make([]message.Message, 0, len(messages))
	start := -1
	for _, msg := range messages {
		orphan := false
		for _, part := range msg.Parts {
			if _, ok := part.(message.ContextSnapshot); ok && msg.ID != summaryID {
				orphan = true
				break
			}
		}
		if orphan {
			continue
		}
		if summaryID != "" && msg.ID == summaryID {
			start = len(active)
		}
		active = append(active, msg)
	}
	if start < 0 {
		return active
	}
	active = active[start:]
	for _, part := range active[0].Parts {
		if snapshot, ok := part.(message.ContextSnapshot); ok {
			expanded, err := snapshot.Expand()
			if err != nil {
				return active
			} // Provider validation rejects an invalid snapshot.
			return append(expanded, active[1:]...)
		}
	}
	active[0].Role = message.User
	return active
}
