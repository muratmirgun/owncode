package chat

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type cacheItem struct {
	width   int
	content []uiMessage
}
type messagesCmp struct {
	app              *app.App
	width, height    int
	viewport         transcriptViewport
	session          session.Session
	messages         []message.Message
	uiMessages       []uiMessage
	currentMsgID     string
	cachedContent    map[string]cacheItem
	spinner          spinner.Model
	rendering        bool
	attachments      viewport.Model
	child            *messagesCmp
	taskHistory      map[string][]message.Message
	revision         uint64
	dirty            bool
	framePending     bool
	animationPending bool
	pendingCommands  []tea.Cmd
	loadGeneration   uint64
	loadCancel       context.CancelFunc
	textBusy         bool
	textGeneration   uint64
	textCache        map[string]renderedText
	textWaiting      map[string]struct{}
}
type renderFinishedMsg struct{}
type activityFrameMsg struct{ owner *messagesCmp }
type conversationFrameMsg struct{ owner *messagesCmp }

type MessageKeys struct {
	PageDown     key.Binding
	PageUp       key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
}

var messageKeys = MessageKeys{
	PageDown: key.NewBinding(
		key.WithKeys("pgdown", "shift+down"),
		key.WithHelp("pgdn/shift+↓", "page down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup", "shift+up"),
		key.WithHelp("pgup/shift+↑", "page up"),
	),
	HalfPageUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "½ page up"),
	),
	HalfPageDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "½ page down"),
	),
}

func (m *messagesCmp) Init() tea.Cmd {
	return m.viewport.Init()
}

func (m *messagesCmp) Update(msg tea.Msg) (model util.Model, command tea.Cmd) {
	defer func() {
		command = tea.Batch(command, m.takeCommands())
		if m.session.ParentSessionID != "" || m.animationPending {
			return
		}
		visible := m
		if m.child != nil {
			visible = m.child
		}
		if visible.ReadingHistory() || !visible.IsAgentWorking() {
			return
		}
		m.animationPending = true
		command = tea.Batch(command, tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return activityFrameMsg{owner: m} }))
	}()
	if frame, ok := msg.(activityFrameMsg); ok && frame.owner == m {
		m.animationPending = false
		m.spinner, _ = m.spinner.Update(m.spinner.Tick())
		if m.child != nil {
			m.child.spinner = m.spinner
		}
		return m, nil
	}

	var cmds []tea.Cmd
	if m.child != nil {
		switch event := msg.(type) {
		case tea.KeyPressMsg:
			if event.String() == "up" || event.String() == "esc" {
				m.child.resetLoads()
				m.child = nil
				if m.dirty {
					m.renderView()
				}
				return m, nil
			}
			_, cmd := m.child.Update(msg)
			return m, cmd
		case tea.MouseClickMsg:
			if event.Button == tea.MouseLeft && event.Y == 0 {
				m.child.resetLoads()
				m.child = nil
				if m.dirty {
					m.renderView()
				}
				return m, nil
			}
			event.Y--
			_, cmd := m.child.Update(event)
			return m, cmd
		case util.ScrollMsg:
			event.Wheel.Y--
			_, cmd := m.child.Update(event)
			return m, cmd
		case tea.MouseWheelMsg:
			event.Y--
			_, cmd := m.child.Update(event)
			return m, cmd
		case SessionSelectedMsg, SessionClearedMsg:
			m.child.resetLoads()
			m.child = nil
		default:
			_, cmd := m.child.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	switch msg := msg.(type) {
	case historyLoadedMsg:
		if msg.owner != m {
			return m, tea.Batch(cmds...)
		}
		return m, m.applyHistory(msg)
	case taskHistoryLoadedMsg:
		if msg.owner == m && msg.generation == m.loadGeneration && m.taskHistory[msg.id] == nil {
			m.taskHistory[msg.id] = msg.messages
			clear(m.cachedContent)
			return m, m.queueRender()
		}
		return m, tea.Batch(cmds...)
	case textRenderedMsg:
		if msg.owner != m {
			return m, tea.Batch(cmds...)
		}
		return m, m.applyText(msg)
	case conversationFrameMsg:
		if msg.owner != m {
			return m, tea.Batch(cmds...)
		}
		m.framePending = false
		if m.dirty && m.child == nil && m.viewport.AtBottom() {
			m.renderView()
		}
		return m, tea.Batch(cmds...)
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || msg.X < 0 || msg.X >= m.width || msg.Y < 0 || msg.Y >= m.viewport.Height() {
			return m, nil
		}
		row := msg.Y + m.viewport.YOffset()
		for _, item := range m.uiMessages {
			if item.taskID != "" && row >= item.position && row < item.position+item.height {
				return m, m.openTask(item.taskID)
			}
		}
		return m, nil
	case dialog.ThemeChangedMsg:
		m.rerender()
		return m, nil
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			cmd := m.SetSession(msg)
			return m, cmd
		}
		return m, nil
	case SessionClearedMsg:
		m.resetLoads()
		m.uiMessages = nil
		m.viewport.SetItems(nil)
		clear(m.cachedContent)
		m.session = session.Session{}
		m.messages = make([]message.Message, 0)
		m.currentMsgID = ""
		m.rendering = false
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "end" {
			m.viewport.GotoBottom()
			if m.dirty {
				m.renderView()
			}
			return m, nil
		}
		if key.Matches(msg, messageKeys.PageUp) || key.Matches(msg, messageKeys.PageDown) ||
			key.Matches(msg, messageKeys.HalfPageUp) || key.Matches(msg, messageKeys.HalfPageDown) {
			u, cmd := m.viewport.Update(msg)
			m.viewport = u
			cmds = append(cmds, cmd)
			if m.viewport.AtBottom() && m.dirty {
				cmds = append(cmds, m.queueRender())
			}
		}

	case util.ScrollMsg:
		if msg.Wheel.X < 0 || msg.Wheel.X >= m.width || msg.Wheel.Y < 0 || msg.Wheel.Y >= m.viewport.Height() {
			return m, nil
		}
		m.viewport, _ = m.viewport.Update(msg)
		if m.viewport.AtBottom() && m.dirty {
			return m, m.queueRender()
		}
		return m, nil
	case tea.MouseWheelMsg:
		if msg.X < 0 || msg.X >= m.width || msg.Y < 0 || msg.Y >= m.viewport.Height() {
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if m.viewport.AtBottom() && m.dirty {
			return m, tea.Batch(cmd, m.queueRender())
		}
		return m, cmd

	case renderFinishedMsg:
		m.rendering = false
		m.viewport.GotoBottom()
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent && msg.Payload.ID == m.session.ID {
			m.session = msg.Payload
			if m.session.SummaryMessageID == m.currentMsgID {
				delete(m.cachedContent, m.currentMsgID)
				m.renderView()
			}
		}
	case pubsub.Event[message.Message]:
		needsRerender := false
		if msg.Payload.SessionID == m.session.ID && msg.Payload.Role == message.Tool {
			// Incremental results update the call's panel, not just the result message.
			for _, parent := range m.messages {
				for _, call := range parent.ToolCalls() {
					for _, result := range msg.Payload.ToolResults() {
						if result.ToolCallID == call.ID {
							delete(m.cachedContent, parent.ID)
							needsRerender = true
						}
					}
				}
			}
		}
		if msg.Payload.SessionID != m.session.ID {
			for _, parent := range m.messages {
				for _, call := range parent.ToolCalls() {
					if call.Name == agent.AgentToolName && agent.TaskID(call) == msg.Payload.SessionID {
						if msg.Payload.Role == message.Assistant {
							m.taskHistory[agent.TaskID(call)] = []message.Message{msg.Payload}
						}
						delete(m.cachedContent, parent.ID)
						needsRerender = true
					}
				}
			}
		}
		if msg.Type == pubsub.CreatedEvent {
			if msg.Payload.SessionID == m.session.ID {

				messageExists := false
				for _, v := range m.messages {
					if v.ID == msg.Payload.ID {
						messageExists = true
						break
					}
				}

				if !messageExists {
					if len(m.messages) > 0 {
						lastMsgID := m.messages[len(m.messages)-1].ID
						delete(m.cachedContent, lastMsgID)
					}

					m.messages = append(m.messages, msg.Payload)
					delete(m.cachedContent, m.currentMsgID)
					m.currentMsgID = msg.Payload.ID
					needsRerender = true
				}
			}
			// There are tool calls from the child task
			for _, v := range m.messages {
				for _, c := range v.ToolCalls() {
					if c.ID == msg.Payload.SessionID {
						delete(m.cachedContent, v.ID)
						needsRerender = true
					}
				}
			}
		} else if msg.Type == pubsub.UpdatedEvent && msg.Payload.SessionID == m.session.ID {
			found := false
			for i, v := range m.messages {
				if v.ID == msg.Payload.ID {
					m.messages[i] = msg.Payload
					found = true
					delete(m.cachedContent, msg.Payload.ID)
					needsRerender = true
					break
				}
			}
			if !found && m.rendering {
				m.messages = append(m.messages, msg.Payload)
				needsRerender = true
			}
		}
		if needsRerender {
			if m.child != nil || !m.viewport.AtBottom() || msg.Payload.SessionID != m.session.ID || (msg.Type == pubsub.UpdatedEvent && msg.Payload.Role == message.Assistant) {
				cmds = append(cmds, m.queueRender())
			} else {
				m.renderView()
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *messagesCmp) IsAgentWorking() bool {
	if m.session.ParentSessionID != "" {
		return agent.IsTaskRunning(m.session.ID)
	}
	return m.app.CoderAgent != nil && m.app.CoderAgent.IsSessionBusy(m.session.ID)
}

func formatTimeDifference(unixTime1, unixTime2 int64) string {
	diffSeconds := float64(math.Abs(float64(unixTime2 - unixTime1)))

	if diffSeconds < 60 {
		return fmt.Sprintf("%.1fs", diffSeconds)
	}

	minutes := int(diffSeconds / 60)
	seconds := int(diffSeconds) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

func (m *messagesCmp) renderView() {
	m.revision++
	m.dirty = false
	followBottom := m.viewport.AtBottom()
	m.uiMessages = make([]uiMessage, 0)
	pos := 0

	if m.width == 0 {
		return
	}
	for inx, msg := range m.messages {
		switch msg.Role {
		case message.User:
			if cache, ok := m.cachedContent[msg.ID]; ok && cache.width == m.width {
				m.uiMessages = append(m.uiMessages, cache.content...)
				continue
			}
			userMsg := renderUserMessage(
				msg,
				msg.ID == m.currentMsgID,
				m.width,
				pos,
				m.textRenderer(msg.ID),
			)
			m.uiMessages = append(m.uiMessages, userMsg)
			m.cachedContent[msg.ID] = cacheItem{
				width:   m.width,
				content: []uiMessage{userMsg},
			}
			pos += userMsg.height + 1 // + 1 for spacing
		case message.Assistant:
			if cache, ok := m.cachedContent[msg.ID]; ok && cache.width == m.width {
				m.uiMessages = append(m.uiMessages, cache.content...)
				continue
			}
			isSummary := m.session.SummaryMessageID == msg.ID
			// Load only on first display; live child events update the card cache.
			for _, call := range msg.ToolCalls() {
				if call.Name != agent.AgentToolName {
					continue
				}
				if _, loaded := m.taskHistory[agent.TaskID(call)]; loaded {
					continue
				}
				id := agent.TaskID(call)
				m.taskHistory[id] = nil
				if m.app.Messages != nil {
					m.pendingCommands = append(m.pendingCommands, m.loadTaskHistory(id))
				}

			}

			assistantMessages := renderAssistantMessage(
				msg,
				inx,
				m.messages,
				m.taskHistory,
				m.currentMsgID,
				isSummary,
				m.width,
				pos,
				m.textRenderer(msg.ID),
			)
			for _, msg := range assistantMessages {
				m.uiMessages = append(m.uiMessages, msg)
				pos += msg.height + 1 // + 1 for spacing
			}
			m.cachedContent[msg.ID] = cacheItem{
				width:   m.width,
				content: assistantMessages,
			}
		}
	}

	m.viewport.SetItems(m.uiMessages)
	for i := range m.uiMessages {
		m.uiMessages[i].position = m.viewport.items[i].start
	}
	if followBottom {
		m.viewport.GotoBottom()
	}
}

func (m *messagesCmp) queueRender() tea.Cmd {
	m.dirty = true
	if m.child != nil || m.framePending || !m.viewport.AtBottom() {
		return nil
	}
	m.framePending = true
	return tea.Tick(time.Second/30, func(time.Time) tea.Msg { return conversationFrameMsg{owner: m} })
}

func (m *messagesCmp) View() string {
	if m.child != nil {
		t := theme.CurrentTheme()
		header := styles.BaseStyle().Width(m.width).Foreground(t.Primary()).Render(ansi.Truncate("↑ Back to main chat · "+m.child.session.Title, m.width, "…"))
		return header + "\n" + m.child.View()
	}
	baseStyle := styles.BaseStyle()

	if m.rendering {
		return baseStyle.
			Width(m.width).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Top,
					"Loading...",
					m.working(),
					m.help(),
				),
			)
	}
	if len(m.messages) == 0 {
		content := baseStyle.
			Width(m.width).
			Height(m.height - 1).
			Render(
				m.initialScreen(),
			)

		return baseStyle.
			Width(m.width).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Top,
					content,
					"",
					m.help(),
				),
			)
	}

	return strings.Join([]string{m.viewport.View(), m.working(), m.help()}, "\n")
}

func hasToolsWithoutResponse(messages []message.Message) bool {
	toolCalls := make([]message.ToolCall, 0)
	toolResults := make([]message.ToolResult, 0)
	for _, m := range messages {
		toolCalls = append(toolCalls, m.ToolCalls()...)
		toolResults = append(toolResults, m.ToolResults()...)
	}

	for _, v := range toolCalls {
		found := false
		for _, r := range toolResults {
			if v.ID == r.ToolCallID {
				found = true
				break
			}
		}
		if !found && v.Finished {
			return true
		}
	}
	return false
}

func hasUnfinishedToolCalls(messages []message.Message) bool {
	toolCalls := make([]message.ToolCall, 0)
	for _, m := range messages {
		toolCalls = append(toolCalls, m.ToolCalls()...)
	}
	for _, v := range toolCalls {
		if !v.Finished {
			return true
		}
	}
	return false
}

func (m *messagesCmp) working() string {
	if m.dirty && !m.viewport.AtBottom() {
		return styles.BaseStyle().Foreground(theme.CurrentTheme().TextMuted()).Width(m.width).Render("Reading history · end latest")
	}
	if !m.IsAgentWorking() {
		return ""
	}
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	label := "Waiting for response"
	if len(m.messages) > 0 {
		last := m.messages[len(m.messages)-1]
		switch {
		case hasToolsWithoutResponse(m.messages):
			label = "Running tools"
		case hasUnfinishedToolCalls(m.messages):
			label = "Preparing tools"
		case last.Role == message.Assistant && !last.IsFinished():
			label = "Thinking"
			if last.Content().Text != "" {
				label = "Responding"
			}
		}
	}
	indicator := base.Foreground(t.Primary()).Render(m.spinner.View())
	text := indicator + base.Foreground(t.TextMuted()).Render("  "+label+"  ·  ") + base.Foreground(t.Text()).Render("esc") + base.Foreground(t.TextMuted()).Render(" interrupt")
	return base.Width(m.width).Render(ansi.Truncate(text, max(1, m.width), "…"))
}

func (m *messagesCmp) help() string { return "" }

func (m *messagesCmp) initialScreen() string {
	baseStyle := styles.BaseStyle()

	return baseStyle.Width(m.width).Render(
		lipgloss.JoinVertical(
			lipgloss.Top,
			baseStyle.Bold(true).Render("Start a conversation"),
			"",
			baseStyle.Foreground(theme.CurrentTheme().TextMuted()).Render("Ask about your code, or type / for commands."),
		),
	)
}

func (m *messagesCmp) rerender() {
	m.textGeneration++
	clear(m.textCache)
	for _, msg := range m.messages {
		delete(m.cachedContent, msg.ID)
	}
	m.renderView()
}

func (m *messagesCmp) SetSize(width, height int) tea.Cmd {
	if m.child != nil {
		m.pendingCommands = append(m.pendingCommands, m.child.SetSize(width, max(1, height-1)))
	}
	if m.width == width && m.height == height {
		return m.takeCommands()
	}
	followBottom := m.viewport.AtBottom()
	m.width = width
	m.height = height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(max(1, height-2))
	m.attachments.SetWidth(width + 40)
	m.attachments.SetHeight(3)
	m.rerender()
	if followBottom {
		m.viewport.GotoBottom()
	}
	return m.takeCommands()
}

func (m *messagesCmp) GetSize() (int, int) {
	return m.width, m.height
}

func (m *messagesCmp) SetSession(selected session.Session) tea.Cmd {
	if m.session.ID == selected.ID {
		return nil
	}
	m.resetLoads()
	m.session = selected
	m.messages = nil
	m.currentMsgID = ""
	m.taskHistory = make(map[string][]message.Message)
	clear(m.cachedContent)
	m.uiMessages = nil
	m.viewport.SetItems(nil)
	m.rendering = true
	return m.loadHistory(false)
}

func (m *messagesCmp) BindingKeys() []key.Binding {
	return []key.Binding{
		m.viewport.KeyMap.PageDown,
		m.viewport.KeyMap.PageUp,
		m.viewport.KeyMap.HalfPageUp,
		m.viewport.KeyMap.HalfPageDown,
	}
}

func NewMessagesCmp(app *app.App) util.Model {
	s := spinner.New()
	s.Spinner = spinner.Spinner{Frames: []string{
		"█▓······", "·█▓·····", "··█▓····", "···█▓···", "····█▓··", "·····█▓·", "······█▓", "▓······█",
	}, FPS: 120 * time.Millisecond}
	vp := newTranscriptViewport()
	attachmets := viewport.New()
	vp.KeyMap.PageUp = messageKeys.PageUp
	vp.KeyMap.PageDown = messageKeys.PageDown
	vp.KeyMap.HalfPageUp = messageKeys.HalfPageUp
	vp.KeyMap.HalfPageDown = messageKeys.HalfPageDown
	return &messagesCmp{
		app:           app,
		cachedContent: make(map[string]cacheItem),
		taskHistory:   make(map[string][]message.Message),
		viewport:      vp,
		spinner:       s,
		attachments:   attachmets,
	}
}

// ScrollOffset returns the active transcript position.
func (m *messagesCmp) ScrollOffset() int {
	if m.child != nil {
		return m.child.ScrollOffset()
	}
	return m.viewport.YOffset()
}

// ReadingHistory reports whether stream updates leave the visible rows frozen.
func (m *messagesCmp) ReadingHistory() bool {
	if m.child != nil {
		return m.child.ReadingHistory()
	}
	return !m.viewport.AtBottom()
}

// ScrollFrameKey changes when the visible transcript or its history hint changes.
func (m *messagesCmp) ScrollFrameKey() string {
	if m.child != nil {
		return m.child.ScrollFrameKey()
	}
	return fmt.Sprintf("%s/%d/%d/%t", m.session.ID, m.revision, m.viewport.YOffset(), m.dirty)
}
