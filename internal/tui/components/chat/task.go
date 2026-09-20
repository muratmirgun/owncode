package chat

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// ReadOnly reports whether the conversation is showing a child task.
func (m *messagesCmp) ReadOnly() bool { return m.child != nil }

func (m *messagesCmp) openTask(id string) tea.Cmd {
	child, err := m.app.Sessions.Get(context.Background(), id)
	if err != nil {
		return util.ReportError(err)
	}
	if child.ParentSessionID != m.session.ID {
		return nil
	}
	history, err := m.app.Messages.List(context.Background(), id)
	if err != nil {
		return util.ReportError(err)
	}
	view := NewMessagesCmp(m.app).(*messagesCmp)
	view.SetSize(m.width, max(1, m.height-1))
	view.session, view.messages = child, history
	if len(history) > 0 {
		view.currentMsgID = history[len(history)-1].ID
	}
	view.renderView()
	view.viewport.GotoBottom()
	m.child = view
	return nil
}
