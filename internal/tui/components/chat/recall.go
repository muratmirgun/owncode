package chat

import (
	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/message"
)

type sentDraft struct{ id, text string }

func (m *editorCmp) resetRecall() {
	m.historyOffset = 0
	m.historyDrafts = nil
}

func (m *editorCmp) rememberMessage(msg message.Message) {
	if msg.Role != message.User || msg.Content().Text == "" {
		return
	}
	for _, previous := range m.history {
		if previous.id == msg.ID {
			return
		}
	}
	// A newly sent message shifts history offsets. Keep the visible draft intact.
	m.resetRecall()
	m.history = append(m.history, sentDraft{id: msg.ID, text: msg.Content().Text})
}

func (m *editorCmp) recallMessage(key tea.KeyPressMsg) bool {
	if !m.textarea.Focused() || len(m.history) == 0 {
		return false
	}
	info := m.textarea.LineInfo()
	next := m.historyOffset
	switch key.String() {
	case "up":
		if m.textarea.Line() != 0 || info.RowOffset != 0 {
			return false
		}
		next = min(len(m.history), next+1)
	case "down":
		if next == 0 || m.textarea.Line() != m.textarea.LineCount()-1 || info.RowOffset < info.Height-1 {
			return false
		}
		next--
	default:
		return false
	}
	if next == m.historyOffset {
		return true
	}
	if m.historyDrafts == nil {
		m.historyDrafts = make(map[int]string)
	}
	m.historyDrafts[m.historyOffset] = m.textarea.Value()
	value, edited := m.historyDrafts[next]
	if !edited && next > 0 {
		value = m.history[len(m.history)-next].text
	}
	m.historyOffset = next
	m.textarea.SetValue(value)
	m.slashIndex = 0
	m.slashDismissed = true
	return true
}
