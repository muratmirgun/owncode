package chat

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func recallFixture() *editorCmp {
	m := &editorCmp{textarea: textarea.New()}
	m.textarea.SetWidth(80)
	m.textarea.Focus()
	for _, id := range []string{"first", "second"} {
		m.rememberMessage(message.Message{ID: id, Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: id}}})
	}
	return m
}
func TestRecallPreservesDraftAndEdits(t *testing.T) {
	m := recallFixture()
	m.textarea.SetValue("unsent draft")
	require.True(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyUp}))
	require.Equal(t, "second", m.textarea.Value())
	m.textarea.SetValue("edited second")
	require.True(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyUp}))
	require.Equal(t, "first", m.textarea.Value())
	require.True(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyDown}))
	require.Equal(t, "edited second", m.textarea.Value())
	require.True(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyDown}))
	require.Equal(t, "unsent draft", m.textarea.Value())
	require.False(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyDown}))
}
func TestRecallLeavesMultilineAndWrappedCursorNavigation(t *testing.T) {
	m := recallFixture()
	m.textarea.SetValue("first line\nsecond line")
	require.False(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyUp}))
	m.textarea.SetWidth(15)
	m.textarea.SetValue(strings.Repeat("a", 40))
	require.False(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyUp}))
	require.False(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}))
}
func TestRecallDeduplicatesAndClearsWithSession(t *testing.T) {
	m := recallFixture()
	m.rememberMessage(message.Message{ID: "second", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "second"}}})
	m.rememberMessage(message.Message{ID: "assistant", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "ignore"}}})
	require.Len(t, m.history, 2)
	m.Update(SessionClearedMsg{})
	require.Empty(t, m.history)
	require.False(t, m.recallMessage(tea.KeyPressMsg{Code: tea.KeyUp}))
}
func TestSlashMenuKeepsArrowPriority(t *testing.T) {
	m := recallFixture()
	m.textarea.SetValue("/")
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Equal(t, "/", m.textarea.Value())
	require.Equal(t, 1, m.slashIndex)
	require.Zero(t, m.historyOffset)
}
