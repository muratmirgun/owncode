package page

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/completions"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/components/chat"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

var ChatPage PageID = "chat"

type chatPage struct {
	app                  *app.App
	editor               layout.Container
	messages             layout.Container
	sidebar              layout.Container
	permissionView       string
	layout               layout.SplitPaneLayout
	session              session.Session
	completionDialog     dialog.CompletionDialog
	showCompletionDialog bool
	slashView            string
}

type ChatKeyMap struct {
	ShowCompletionDialog key.Binding
	NewSession           key.Binding
	Cancel               key.Binding
}

var keyMap = ChatKeyMap{
	ShowCompletionDialog: key.NewBinding(
		key.WithKeys("@"),
		key.WithHelp("@", "Complete"),
	),
	NewSession: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "new session"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

func (p *chatPage) Init() tea.Cmd {
	cmds := []tea.Cmd{
		p.layout.Init(),
		p.completionDialog.Init(),
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case chat.PermissionPanelMsg:
		p.permissionView = string(msg)
		cmd := p.layout.SetBottomAccessory(string(msg))
		w, h := p.layout.GetSize()
		return p, tea.Batch(cmd, p.SetSize(w, h))
	case chat.SlashSuggestionsMsg:
		p.slashView = msg.View
		return p, nil
	case chat.SessionClearedMsg:
		p.session = session.Session{}
	case tea.WindowSizeMsg:
		cmd := p.layout.SetSize(msg.Width, msg.Height)
		cmds = append(cmds, cmd)
	case dialog.CompletionDialogCloseMsg:
		p.showCompletionDialog = false
	case chat.SendMsg:
		cmd := p.sendMessage(msg.Text, msg.Attachments)
		if cmd != nil {
			return p, cmd
		}
	case dialog.CommandRunCustomMsg:
		// Check if the agent is busy before executing custom commands
		if p.app.CoderAgent.IsBusy() {
			return p, util.ReportWarn("Agent is busy, please wait before executing a command...")
		}

		// Process the command content with arguments if any
		content := msg.Content
		if msg.Args != nil {
			// Replace all named arguments with their values
			for name, value := range msg.Args {
				placeholder := "$" + name
				content = strings.ReplaceAll(content, placeholder, value)
			}
		}

		// Handle custom command execution
		cmd := p.sendMessage(content, nil)
		if cmd != nil {
			return p, cmd
		}
	case chat.SessionSelectedMsg:
		p.session = msg
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyMap.ShowCompletionDialog):
			p.showCompletionDialog = true
			// Continue sending keys to layout->chat
		case key.Matches(msg, keyMap.NewSession) && p.slashView == "":
			p.session = session.Session{}
			return p, tea.Batch(
				util.CmdHandler(chat.SessionClearedMsg{}),
			)
		case key.Matches(msg, keyMap.Cancel) && p.slashView == "":
			if p.session.ID != "" {
				// Cancel the current session's generation process
				// This allows users to interrupt long-running operations
				p.app.CoderAgent.Cancel(p.session.ID)
				return p, nil
			}
		}
	}
	if p.showCompletionDialog {
		context, contextCmd := p.completionDialog.Update(msg)
		p.completionDialog = context.(dialog.CompletionDialog)
		cmds = append(cmds, contextCmd)

		// Doesn't forward event if enter key is pressed
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				return p, tea.Batch(cmds...)
			}
		}
	}

	u, cmd := p.layout.Update(msg)
	cmds = append(cmds, cmd)
	p.layout = u.(layout.SplitPaneLayout)
	switch msg.(type) {
	case tea.WindowSizeMsg, tea.KeyMsg, chat.SessionSelectedMsg, chat.SessionClearedMsg, dialog.AttachmentAddedMsg, dialog.CompletionSelectedMsg:
		w, h := p.layout.GetSize()
		cmds = append(cmds, p.SetSize(w, h))
	}

	return p, tea.Batch(cmds...)
}

func (p *chatPage) sendMessage(text string, attachments []message.Attachment) tea.Cmd {
	if p.app.CoderAgent.Model().ID == "" {
		return util.ReportWarn(agent.ErrNotConfigured.Error())
	}

	var cmds []tea.Cmd
	if p.session.ID == "" {
		session, err := p.app.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return util.ReportError(err)
		}

		p.session = session
		cmds = append(cmds, util.CmdHandler(chat.SessionSelectedMsg(session)))
	}

	_, err := p.app.CoderAgent.Run(context.Background(), p.session.ID, text, attachments...)
	if err != nil {
		return util.ReportError(err)
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) SetSize(width, height int) tea.Cmd {
	cmd := p.layout.SetSize(width, height)
	p.editor.Update(chat.HomeEditorMsg(p.isHome()))
	if p.isHome() {
		editorWidth := max(4, min(88, width-8))
		editorHeight := 3
		if preferred, ok := p.editor.(interface{ PreferredHeight(int) int }); ok {
			editorHeight = preferred.PreferredHeight(editorWidth)
		}
		p.editor.SetSize(editorWidth, min(max(3, height/3), editorHeight))
	}
	return cmd
}

func (p *chatPage) GetSize() (int, int) {
	return p.layout.GetSize()
}

func (p *chatPage) View() string {
	layoutView := p.layout.View()
	editorX := 0
	_, layoutHeight := p.layout.GetSize()
	_, editorHeight := p.editor.GetSize()
	editorY := layoutHeight - editorHeight
	if p.isHome() {
		layoutView, editorX, editorY = p.homeView()
	}

	if p.showCompletionDialog {
		editorWidth, _ := p.editor.GetSize()

		p.completionDialog.SetWidth(editorWidth)
		overlay := p.completionDialog.View()

		layoutView = layout.PlaceOverlay(
			editorX,
			max(0, editorY-lipgloss.Height(overlay)),
			overlay,
			layoutView,
			false,
		)
	}

	if p.slashView != "" {
		layoutView = layout.PlaceOverlay(editorX, max(0, editorY-lipgloss.Height(p.slashView)), p.slashView, layoutView, false)
	}
	return layoutView
}

func (p *chatPage) isHome() bool {
	return p.session.ID == "" && p.permissionView == ""
}

func (p *chatPage) homeView() (string, int, int) {
	width, height := p.layout.GetSize()
	editorWidth, _ := p.editor.GetSize()
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	logo := homeLogo(editorWidth)
	keyStyle := base.Foreground(t.Secondary()).Bold(true)
	labelStyle := base.Foreground(t.TextMuted())
	hintsText := " " + keyStyle.Render("/") + labelStyle.Render(" commands   ") +
		keyStyle.Render("ctrl+o") + labelStyle.Render(" models   ") +
		keyStyle.Render("ctrl+s") + labelStyle.Render(" sessions")
	hints := base.Width(editorWidth).Render(ansi.Truncate(hintsText, editorWidth, "…"))
	tipText := base.Foreground(t.Warning()).Bold(true).Render("● Tip  ") +
		base.Foreground(t.Text()).Render("Use ") + keyStyle.Render("@") +
		labelStyle.Render(" to add files to your message")
	tip := base.Width(editorWidth).Align(lipgloss.Center).Render(ansi.Truncate(tipText, editorWidth, "…"))

	content := lipgloss.JoinVertical(lipgloss.Left, logo, "", "", p.editor.View(), hints, "", "", tip)
	x := max(0, (width-editorWidth)/2)
	y := max(0, (height-lipgloss.Height(content))/2)
	background := base.Width(width).Height(height).Render("")
	view := layout.PlaceOverlay(x, y, content, background, false)
	return styles.Surface(view, t.Background()), x, y + lipgloss.Height(logo) + 2
}

func homeLogo(width int) string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	if width < 46 {
		return base.Foreground(t.Text()).Bold(true).Width(width).Align(lipgloss.Center).Render("owncode")
	}
	glyphs := map[rune][]string{
		'o': {"█████", "█   █", "█   █", "█   █", "█████"},
		'w': {"█   █", "█   █", "█ █ █", "█ █ █", "█████"},
		'n': {"█████", "█   █", "█   █", "█   █", "█   █"},
		'c': {"█████", "█    ", "█    ", "█    ", "█████"},
		'd': {"    █", "    █", "█████", "█   █", "█████"},
		'e': {"█████", "█   █", "█████", "█    ", "█████"},
	}
	var lines []string
	for row := range 5 {
		var line strings.Builder
		for i, letter := range "owncode" {
			color := t.TextMuted()
			if i >= 3 {
				color = t.Text()
			}
			line.WriteString(base.Foreground(color).Render(glyphs[letter][row]))
			if i < 6 {
				line.WriteByte(' ')
			}
		}
		lines = append(lines, base.Width(width).Align(lipgloss.Center).Render(line.String()))
	}
	return strings.Join(lines, "\n")
}

func (p *chatPage) BindingKeys() []key.Binding {
	bindings := layout.KeyMapToSlice(keyMap)
	bindings = append(bindings, p.messages.BindingKeys()...)
	bindings = append(bindings, p.editor.BindingKeys()...)
	return bindings
}

func NewChatPage(app *app.App) tea.Model {
	cg := completions.NewFileAndFolderContextGroup()
	completionDialog := dialog.NewCompletionDialogCmp(cg)

	messagesContainer := layout.NewContainer(
		chat.NewMessagesCmp(app),
		layout.WithPadding(1, 1, 0, 1),
	)
	editorContainer := layout.NewContainer(
		chat.NewEditorCmp(app),
		layout.WithBorder(false, false, false, true),
		layout.WithSecondarySurface(),
	)
	sidebarContainer := layout.NewContainer(
		chat.NewSidebarCmp(session.Session{}, app.History),
		layout.WithPadding(1, 2, 1, 2),
		layout.WithBorder(false, false, false, true),
		layout.WithSecondarySurface(),
	)
	return &chatPage{
		app:              app,
		editor:           editorContainer,
		messages:         messagesContainer,
		sidebar:          sidebarContainer,
		completionDialog: completionDialog,
		layout: layout.NewSplitPane(
			layout.WithLeftPanel(messagesContainer),
			layout.WithRightPanel(sidebarContainer),
			layout.WithBottomPanel(editorContainer),
		),
	}
}
