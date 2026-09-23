package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/logging"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// ToggleProfileMsg requests a switch between Build and Plan.
type ToggleProfileMsg struct{}

type editorCmp struct {
	history           []sentDraft
	historyOffset     int
	historyDrafts     map[int]string
	home              bool
	width             int
	height            int
	app               *app.App
	session           session.Session
	textarea          textarea.Model
	attachments       []message.Attachment
	deleteMode        bool
	skillCommands     []slashCommand
	slashIndex        int
	slashDismissed    bool
	throughput        map[string]*throughputStats
	throughputParents map[string]string
	throughputTicking bool
}

type EditorKeyMaps struct {
	Send       key.Binding
	OpenEditor key.Binding
}

type bluredEditorKeyMaps struct {
	Send       key.Binding
	Focus      key.Binding
	OpenEditor key.Binding
}
type DeleteAttachmentKeyMaps struct {
	AttachmentDeleteMode key.Binding
	Escape               key.Binding
	DeleteAllAttachments key.Binding
}

var editorMaps = EditorKeyMaps{
	Send: key.NewBinding(
		key.WithKeys("enter", "ctrl+s"),
		key.WithHelp("enter", "send message"),
	),
	OpenEditor: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "open editor"),
	),
}

var DeleteKeyMaps = DeleteAttachmentKeyMaps{
	AttachmentDeleteMode: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r+{i}", "delete attachment at index i"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel delete mode"),
	),
	DeleteAllAttachments: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("ctrl+r+r", "delete all attchments"),
	),
}

const (
	maxAttachments = 5
)

func (m *editorCmp) openEditor() tea.Cmd {
	if m.app.CoderAgent.Model().ID == "" {
		return util.ReportWarn(agent.ErrNotConfigured.Error())
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	tmpfile, err := os.CreateTemp("", "msg_*.md")
	if err != nil {
		return util.ReportError(err)
	}
	tmpfile.Close()
	c := exec.Command(editor, tmpfile.Name()) //nolint:gosec
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return util.ReportError(err)
		}
		content, err := os.ReadFile(tmpfile.Name())
		if err != nil {
			return util.ReportError(err)
		}
		if len(content) == 0 {
			return util.ReportWarn("Message is empty")
		}
		os.Remove(tmpfile.Name())
		attachments := m.attachments
		m.attachments = nil
		return SendMsg{
			Text:        string(content),
			Attachments: attachments,
		}
	})
}

func (m *editorCmp) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.loadSkillNames())
}

func (m *editorCmp) send() tea.Cmd {
	if m.app.CoderAgent.Model().ID == "" {
		return util.ReportWarn(agent.ErrNotConfigured.Error())
	}

	if m.app.CoderAgent.IsSessionBusy(m.session.ID) {
		return util.ReportWarn("Agent is working, please wait...")
	}

	value := m.textarea.Value()
	m.textarea.Reset()
	m.resetRecall()
	attachments := m.attachments

	m.attachments = nil
	if value == "" {
		return nil
	}
	return tea.Batch(
		util.CmdHandler(SendMsg{
			Text:        value,
			Attachments: attachments,
		}),
	)
}

// InsertTextMsg appends reviewed content without submitting the draft.
type InsertTextMsg string

func (m *editorCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case skillNamesMsg:
		if msg.owner == m {
			m.skillCommands = msg.commands
		}
		return m, m.slashSuggestions()
	case dialog.SkillsChangedMsg:
		return m, m.loadSkillNames()
	case InsertTextMsg:
		m.textarea.SetValue(m.textarea.Value() + string(msg))
		return m, nil
	case HomeEditorMsg:
		m.home = bool(msg)
		return m, nil
	case dialog.ThemeChangedMsg:
		m.textarea = CreateTextArea(&m.textarea)
	case dialog.CompletionSelectedMsg:
		existingValue := m.textarea.Value()
		modifiedValue := strings.Replace(existingValue, msg.SearchString, msg.CompletionValue, 1)

		m.textarea.SetValue(modifiedValue)
		return m, nil
	case throughputTickMsg:
		active := false
		for _, stats := range m.throughput {
			if stats.messageID != "" && !stats.finished {
				stats.refresh(time.Time(msg))
				active = true
			}
		}
		m.throughputTicking = active
		if active {
			return m, throughputTick()
		}
		return m, nil
	case pubsub.Event[message.Message]:
		if msg.Type == pubsub.DeletedEvent {
			if stats := m.throughput[msg.Payload.SessionID]; stats != nil && stats.messageID == msg.Payload.ID {
				delete(m.throughput, msg.Payload.SessionID)
			}
			return m, nil
		}
		if msg.Payload.Role == message.User && msg.Payload.SessionID == m.session.ID && msg.Type == pubsub.CreatedEvent {
			m.rememberMessage(msg.Payload)
		}
		if msg.Payload.Role == message.Assistant && msg.Type != pubsub.DeletedEvent {
			m.rememberWorkerParents(msg.Payload)
			if m.throughput == nil {
				m.throughput = make(map[string]*throughputStats)
			}
			stats := m.throughput[msg.Payload.SessionID]
			if stats == nil {
				stats = &throughputStats{}
				m.throughput[msg.Payload.SessionID] = stats
			}
			stats.observe(msg.Payload, time.Now())
			if stats.messageID != "" && !stats.finished && !m.throughputTicking {
				m.throughputTicking = true
				return m, throughputTick()
			}
		}
		return m, nil
	case pubsub.Event[session.Session]:
		if m.throughputParents == nil {
			m.throughputParents = make(map[string]string)
		}
		if msg.Type == pubsub.DeletedEvent {
			delete(m.throughputParents, msg.Payload.ID)
			delete(m.throughput, msg.Payload.ID)
		} else {
			m.throughputParents[msg.Payload.ID] = msg.Payload.ParentSessionID
		}
		return m, nil
	case historyLoadedMsg:
		if msg.err == nil && msg.session.ID == m.session.ID && msg.owner != nil && msg.generation == msg.owner.loadGeneration {
			m.history = nil
			for _, previous := range msg.owner.messages {
				m.rememberMessage(previous)
				m.rememberWorkerParents(previous)
			}
		}
		return m, nil
	case SessionClearedMsg:
		m.history = nil
		m.resetRecall()
		m.session = session.Session{}
		m.textarea.Reset()
		m.attachments = nil
		m.slashDismissed = false
		return m, m.slashSuggestions()
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			m.session = msg
			m.history = nil
			m.resetRecall()
		}
		return m, nil
	case dialog.AttachmentAddedMsg:
		if len(m.attachments) >= maxAttachments {
			logging.ErrorPersist(fmt.Sprintf("cannot add more than %d images", maxAttachments))
			return m, cmd
		}
		m.attachments = append(m.attachments, msg.Attachment)
	case tea.KeyPressMsg:
		if handled, cmd := m.handleSlash(msg); handled {
			return m, cmd
		}
		if msg.String() == "tab" && m.textarea.Focused() && !m.deleteMode {
			return m, util.CmdHandler(ToggleProfileMsg{})
		}
		if !m.deleteMode && m.recallMessage(msg) {
			return m, m.slashSuggestions()
		}
		if key.Matches(msg, DeleteKeyMaps.AttachmentDeleteMode) {
			m.deleteMode = true
			return m, nil
		}
		if key.Matches(msg, DeleteKeyMaps.DeleteAllAttachments) && m.deleteMode {
			m.deleteMode = false
			m.attachments = nil
			return m, nil
		}
		if m.deleteMode && len(msg.Text) > 0 && unicode.IsDigit([]rune(msg.Text)[0]) {
			num := int([]rune(msg.Text)[0] - '0')
			m.deleteMode = false
			if num < 10 && len(m.attachments) > num {
				if num == 0 {
					m.attachments = m.attachments[num+1:]
				} else {
					m.attachments = slices.Delete(m.attachments, num, num+1)
				}
				return m, nil
			}
		}
		if key.Matches(msg, messageKeys.PageUp) || key.Matches(msg, messageKeys.PageDown) ||
			key.Matches(msg, messageKeys.HalfPageUp) || key.Matches(msg, messageKeys.HalfPageDown) {
			return m, nil
		}
		if key.Matches(msg, editorMaps.OpenEditor) {
			if m.app.CoderAgent.IsSessionBusy(m.session.ID) {
				return m, util.ReportWarn("Agent is working, please wait...")
			}
			return m, m.openEditor()
		}
		if key.Matches(msg, DeleteKeyMaps.Escape) {
			m.deleteMode = false
			return m, nil
		}
		// Hanlde Enter key
		if m.textarea.Focused() && key.Matches(msg, editorMaps.Send) {
			value := m.textarea.Value()
			if len(value) > 0 && value[len(value)-1] == '\\' {
				// If the last character is a backslash, remove it and add a newline
				m.textarea.SetValue(value[:len(value)-1] + "\n")
				return m, nil
			} else {
				// Otherwise, send the message
				return m, m.send()
			}
		}

	}
	before := m.textarea.Value()
	m.textarea, cmd = m.textarea.Update(msg)
	if before != m.textarea.Value() {
		m.slashIndex = 0
		m.slashDismissed = false
		return m, tea.Batch(cmd, m.slashSuggestions())
	}
	return m, cmd
}

// Worker calls also identify resumed children before their next session update.
func (m *editorCmp) rememberWorkerParents(msg message.Message) {
	for _, call := range msg.ToolCalls() {
		if call.Name != agent.AgentToolName {
			continue
		}
		var params struct {
			WorkerID string `json:"worker_id"`
		}
		if json.Unmarshal([]byte(call.Input), &params) != nil {
			continue
		}
		id := call.ID
		if params.WorkerID != "" {
			id = params.WorkerID
		}
		if m.throughputParents == nil {
			m.throughputParents = make(map[string]string)
		}
		m.throughputParents[id] = msg.SessionID
	}
}

func (m *editorCmp) View() string {
	bg := theme.CurrentTheme().BackgroundSecondary()
	content := m.editorView()
	if m.home && len(m.attachments) == 0 {
		t := theme.CurrentTheme()
		_, profile := config.CurrentProfile()
		color := t.Secondary()
		if profile.ReadOnly {
			color = t.Warning()
		}
		border := styles.BaseStyle().Background(bg).Foreground(color).Render("│ ")
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			lines[i] = border + ansi.Truncate(line, max(1, m.width-2), "…")
		}
		content = strings.Join(lines, "\n")
	}
	view := styles.BaseStyle().Background(bg).Width(m.width).Height(m.height).Render(content)
	return styles.Surface(view, bg)
}

func (m *editorCmp) editorView() string {
	t := theme.CurrentTheme()
	if m.home {
		m.textarea.Placeholder = `Ask anything… "Fix a TODO in the codebase"`
	} else {
		m.textarea.Placeholder = "Message… (/ commands, @ files)"
	}

	// Style the prompt with theme colors
	style := lipgloss.NewStyle().
		Padding(0, 0, 0, 1).
		Bold(true).
		Foreground(t.Primary()).Background(t.BackgroundSecondary())

	if m.home && len(m.attachments) == 0 {
		m.resizeTextarea(max(1, m.height-3))
		return lipgloss.JoinVertical(lipgloss.Left, "", m.textarea.View(), "", " "+m.homeProfileView())
	}
	m.resizeTextarea(max(1, m.height-2))
	if len(m.attachments) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, m.throughputView(), "", lipgloss.JoinHorizontal(lipgloss.Top, style.Render(">"), m.textarea.View()))
	}
	m.resizeTextarea(max(1, m.height-3))
	return lipgloss.JoinVertical(lipgloss.Top,
		m.throughputView(),
		"",
		m.attachmentsContent(),
		lipgloss.JoinHorizontal(lipgloss.Top, style.Render(">"),
			m.textarea.View()),
	)
}

func (m *editorCmp) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	m.textarea.SetWidth(max(1, width-2))
	m.resizeTextarea(max(1, height-2))
	return nil
}

func (m *editorCmp) GetSize() (int, int) {
	return m.textarea.Width(), m.textarea.Height()
}

func (m *editorCmp) attachmentsContent() string {
	var styledAttachments []string
	t := theme.CurrentTheme()
	attachmentStyles := styles.BaseStyle().
		MarginLeft(1).
		Background(t.TextMuted()).
		Foreground(t.Text())
	for i, attachment := range m.attachments {
		var filename string
		if len(attachment.FileName) > 10 {
			filename = fmt.Sprintf(" %s %s...", styles.DocumentIcon, attachment.FileName[0:7])
		} else {
			filename = fmt.Sprintf(" %s %s", styles.DocumentIcon, attachment.FileName)
		}
		if m.deleteMode {
			filename = fmt.Sprintf("%d%s", i, filename)
		}
		styledAttachments = append(styledAttachments, attachmentStyles.Render(filename))
	}
	content := lipgloss.JoinHorizontal(lipgloss.Left, styledAttachments...)
	return content
}

func (m *editorCmp) BindingKeys() []key.Binding {
	bindings := []key.Binding{}
	bindings = append(bindings, layout.KeyMapToSlice(editorMaps)...)
	bindings = append(bindings, layout.KeyMapToSlice(DeleteKeyMaps)...)
	bindings = append(bindings,
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "previous prompt at first line")),
		key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next prompt at last line")),
	)
	return bindings
}

func CreateTextArea(existing *textarea.Model) textarea.Model {
	t := theme.CurrentTheme()
	bgColor := t.BackgroundSecondary()
	textColor := t.Text()

	ta := textarea.New()
	ta.SetVirtualCursor(true)
	taStyles := ta.Styles()
	taStyles.Blurred.Base = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	taStyles.Blurred.CursorLine = styles.BaseStyle().Background(bgColor)
	taStyles.Blurred.Placeholder = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	taStyles.Blurred.Text = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	taStyles.Focused.Base = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	taStyles.Focused.CursorLine = styles.BaseStyle().Background(bgColor)
	taStyles.Focused.Placeholder = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	taStyles.Focused.Text = styles.BaseStyle().Background(bgColor).Foreground(textColor)

	taStyles.Focused.EndOfBuffer = styles.BaseStyle().Background(bgColor)
	taStyles.Blurred.EndOfBuffer = styles.BaseStyle().Background(bgColor)
	taStyles.Focused.Prompt = styles.BaseStyle().Background(bgColor)
	taStyles.Blurred.Prompt = styles.BaseStyle().Background(bgColor)
	taStyles.Cursor.Color = textColor
	ta.SetStyles(taStyles)
	ta.Prompt = " "
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.Placeholder = "Message… (/ commands, @ files)"

	if existing != nil {
		ta.SetValue(existing.Value())
		ta.SetWidth(existing.Width())
		ta.SetHeight(existing.Height())
	}

	ta.Focus()
	return ta
}

func NewEditorCmp(app *app.App) util.Model {
	ta := CreateTextArea(nil)
	return &editorCmp{
		app:      app,
		textarea: ta,
	}
}

// PreferredHeight grows with explicit and wrapped lines, then scrolls at eight.
func (m *editorCmp) PreferredHeight(width int) int {
	textWidth := max(1, width-3)
	lines := strings.Count(ansi.Wrap(m.textarea.Value(), textWidth, ""), "\n") + 1
	extra := 2 // Throughput row and separation from the draft.
	if m.home {
		extra++
	}
	if len(m.attachments) > 0 {
		extra++
	}
	return min(8, max(1, lines)) + extra
}

// resizeTextarea reveals the complete draft when the expanded area can fit it.
func (m *editorCmp) resizeTextarea(height int) {
	previous := m.textarea.Height()
	m.textarea.SetHeight(height)
	if height <= previous {
		return
	}
	lines := strings.Count(ansi.Wrap(m.textarea.Value(), max(1, m.textarea.Width()-1), ""), "\n") + 1
	if lines > height {
		return
	}
	row := m.textarea.Line()
	info := m.textarea.LineInfo()
	column := info.StartColumn + info.ColumnOffset
	m.textarea.MoveToBegin()
	for m.textarea.Line() < row {
		m.textarea.CursorDown()
	}
	m.textarea.SetCursorColumn(column)
}
