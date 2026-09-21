package chat

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// ReadOnly reports whether the conversation is showing a child task.
func (m *messagesCmp) ReadOnly() bool { return m.child != nil }

type historyLoadedMsg struct {
	owner      *messagesCmp
	generation uint64
	session    session.Session
	messages   []message.Message
	err        error
}
type taskHistoryLoadedMsg struct {
	owner      *messagesCmp
	generation uint64
	id         string
	messages   []message.Message
}

func (m *messagesCmp) resetLoads() {
	if m.loadCancel != nil {
		m.loadCancel()
	}
	m.loadGeneration++
	m.textGeneration++
	clear(m.textCache)
	clear(m.textWaiting)
}

func (m *messagesCmp) loadHistory(child bool) tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	m.loadCancel = cancel
	selected, generation, app := m.session, m.loadGeneration, m.app
	return func() tea.Msg {
		defer cancel()
		result := historyLoadedMsg{owner: m, generation: generation, session: selected}
		if child {
			result.session, result.err = app.Sessions.Get(ctx, selected.ID)
			if result.err != nil {
				return result
			}
			if result.session.ParentSessionID != selected.ParentSessionID {
				result.session = selected
				result.err = errors.New("worker does not belong to this session")
				return result
			}
		}
		result.messages, result.err = app.Messages.List(ctx, selected.ID)
		return result
	}
}

func (m *messagesCmp) applyHistory(result historyLoadedMsg) tea.Cmd {
	if result.owner != m || result.generation != m.loadGeneration {
		return nil
	}
	m.rendering = false
	if result.err != nil {
		return util.ReportError(result.err)
	}
	// Events received during the read are newer than the stored snapshot.
	latest := make(map[string]message.Message, len(m.messages))
	for _, msg := range m.messages {
		latest[msg.ID] = msg
	}
	merged := make([]message.Message, 0, len(result.messages)+len(latest))
	for _, msg := range result.messages {
		if newer, ok := latest[msg.ID]; ok {
			msg = newer
			delete(latest, msg.ID)
		}
		merged = append(merged, msg)
	}
	for _, msg := range m.messages {
		if _, ok := latest[msg.ID]; ok {
			merged = append(merged, msg)
		}
	}
	m.session, m.messages = result.session, merged
	if len(merged) > 0 {
		m.currentMsgID = merged[len(merged)-1].ID
	}
	clear(m.cachedContent)
	m.renderView()
	m.viewport.GotoBottom()
	return nil
}

func (m *messagesCmp) openTask(id string) tea.Cmd {
	if m.child != nil {
		m.child.resetLoads()
	}
	view := NewMessagesCmp(m.app).(*messagesCmp)
	view.SetSize(m.width, max(1, m.height-1))
	view.session = session.Session{ID: id, ParentSessionID: m.session.ID, Title: "Loading worker…"}
	view.rendering = true
	m.child = view
	return view.loadHistory(true)
}

func (m *messagesCmp) loadTaskHistory(id string) tea.Cmd {
	generation, service := m.loadGeneration, m.app.Messages
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		history, _ := service.List(ctx, id)
		var latest []message.Message
		for _, msg := range history {
			if msg.Role == message.Assistant {
				latest = []message.Message{msg}
			}
		}
		return taskHistoryLoadedMsg{owner: m, generation: generation, id: id, messages: latest}
	}
}

func (m *messagesCmp) takeCommands() tea.Cmd {
	commands := m.pendingCommands
	m.pendingCommands = nil
	return tea.Batch(commands...)
}
