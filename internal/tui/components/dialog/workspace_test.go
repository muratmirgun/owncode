package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
)

func TestAgentRowsResolveToolResults(t *testing.T) {
	msgs := []message.Message{
		{Parts: []message.ContentPart{
			message.ToolCall{ID: "one", Name: "agent", Input: `{"prompt":"Find the auth handler","role":"explore"}`},
			message.ToolCall{ID: "two", Name: "agent", Input: `{"prompt":"Review cancellation","role":"review"}`},
			message.ToolCall{ID: "three", Name: "agent", Input: `{"prompt":"Old task"}`},
		}},
		{Parts: []message.ContentPart{message.ToolResult{ToolCallID: "one", Content: "Found auth.go", IsError: true}}},
	}
	rows := collectAgentRows(msgs, func(id string) bool { return id == "two" })
	require.Len(t, rows, 3)
	require.Equal(t, "Failed", rows[0].state)
	require.Contains(t, rows[0].detail, "Found auth.go")
	require.Equal(t, "Working", rows[1].state)
	require.Equal(t, "Interrupted", rows[2].state)
}

func TestWorkspacePanelsRender(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfg, err := config.Load(dir, false)
	require.NoError(t, err)
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	cfg.Compaction = config.CompactionSettings{Mode: "handoff", Focus: "Keep file paths, decisions and failing tests", Threshold: 80}
	cfg.AutoCompact = true
	panels := map[string]string{}
	for _, size := range [][2]int{{110, 32}, {60, 24}} {
		settings := &settingsCmp{tab: 2, width: size[0], height: size[1]}
		view := settings.View()
		require.LessOrEqual(t, lipgloss.Width(view), size[0])
		require.LessOrEqual(t, lipgloss.Height(view), size[1])
		require.Contains(t, view, "Summary mode")
		if size[0] == 110 {
			panels["settings"] = view
		}
		agents := NewAgentsCmp(nil).(*agentsCmp)
		agents.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		agents.rows = []agentRow{
			{id: "1", title: "explore · Map the compaction pipeline", state: "Done", tokens: 8240, cost: 0.012, detail: "Task\nMap the compaction pipeline\n\nResult\nSettings pass the selected summary mode and focus to the provider. The transcript stays in session storage."},
			{id: "2", title: "review · Check cancellation and context boundaries", state: "Working", tokens: 3150, cost: 0.006},
			{id: "3", title: "explore · Find terminal layout tests", state: "Done", tokens: 1800, cost: 0.002},
		}
		view = agents.View()
		require.LessOrEqual(t, lipgloss.Width(view), size[0])
		require.LessOrEqual(t, lipgloss.Height(view), size[1])
		if size[0] == 110 {
			panels["agents"] = view
		}
	}
	if output := os.Getenv("OWNCODE_PREVIEW_DIR"); output != "" {
		require.NoError(t, os.MkdirAll(output, 0755))
		for name, view := range panels {
			require.NoError(t, os.WriteFile(filepath.Join(output, name+".ansi"), []byte(view), 0600))
		}
	}
	// Focus editing never leaks text into the settings search.
	settings := &settingsCmp{tab: 2, editingFocus: true, focusDraft: "Keep"}
	settings.Update(tea.PasteMsg{Content: " tests\nand paths"})
	require.True(t, strings.Contains(settings.focusDraft, "tests and paths"))
	require.Empty(t, settings.query)
	settings.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.False(t, settings.editingFocus)
}

func TestAgentsShowWorkerSettingsAndPreserveListHeight(t *testing.T) {
	name := strings.Repeat("worker-model-", 8)
	for _, size := range [][2]int{{60, 24}, {110, 32}} {
		a := NewAgentsCmp(nil).(*agentsCmp)
		a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for i := 0; i < 20; i++ {
			a.rows = append(a.rows, agentRow{title: "Review source", state: "Working", metadata: &message.Message{Role: message.Assistant, Model: models.ModelID(name), ReasoningEffort: "high"}, detail: "Task: review"})
		}
		a.selected = 19
		list := a.View()
		require.Contains(t, ansi.Strip(list), "Effort: high")
		require.LessOrEqual(t, lipgloss.Width(list), size[0])
		require.LessOrEqual(t, lipgloss.Height(list), size[1])
		a.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.True(t, a.details)
		detail := ansi.Strip(a.View())
		require.Contains(t, detail, "Reasoning: high")
		require.Contains(t, agentDetail(a.rows[19]), "Model: "+name)
		updated := append([]agentRow(nil), a.rows...)
		replacement := *updated[19].metadata
		replacement.ReasoningEffort = "low"
		updated[19].metadata = &replacement
		a.Update(agentRowsMsg{parent: a.parent, generation: a.generation, rows: updated})
		require.Contains(t, ansi.Strip(a.View()), "Reasoning: low")
	}
}

func TestAgentQueueAndStableSelection(t *testing.T) {
	messages := []message.Message{{Role: message.Assistant, Parts: []message.ContentPart{
		message.ToolCall{ID: "queued", Name: "agent", Input: `{"prompt":"Wait"}`, Finished: true, Execution: "queued"},
		message.ToolCall{ID: "running", Name: "agent", Input: `{"prompt":"Work"}`, Finished: true, Execution: "running"},
	}}}
	rows := collectAgentRows(messages, func(id string) bool { return id == "running" }, true)
	require.Equal(t, "Queued", rows[0].state)
	a := NewAgentsCmp(nil).(*agentsCmp)
	a.rows = rows
	a.parent = "parent"
	a.selected = 0
	a.Update(agentRowsMsg{parent: "parent", rows: rows})
	require.Equal(t, "running", a.rows[0].id)
	require.Equal(t, "queued", a.rows[a.selected].id)
}
