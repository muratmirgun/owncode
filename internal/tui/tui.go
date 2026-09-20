package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/logging"
	"github.com/muratmirgun/owncode/internal/permission"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/components/chat"
	"github.com/muratmirgun/owncode/internal/tui/components/core"
	"github.com/muratmirgun/owncode/internal/tui/components/dialog"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/page"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type keyMap struct {
	Logs          key.Binding
	Quit          key.Binding
	Help          key.Binding
	SwitchSession key.Binding
	Commands      key.Binding
	Filepicker    key.Binding
	Models        key.Binding
	SwitchTheme   key.Binding
	Reasoning     key.Binding
}

type startCompactSessionMsg struct{}

const (
	quitKey = "q"
)

var keys = keyMap{
	Reasoning: key.NewBinding(key.WithKeys("alt+r", "f4"), key.WithHelp("f4 / alt+r", "cycle reasoning")),
	Logs: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "logs"),
	),

	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("ctrl+_", "ctrl+h"),
		key.WithHelp("ctrl+?", "toggle help"),
	),

	SwitchSession: key.NewBinding(
		key.WithKeys("ctrl+s", "f3"),
		key.WithHelp("f3 / ctrl+s", "switch session"),
	),

	Commands: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "commands"),
	),
	Filepicker: key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "select files to upload"),
	),
	Models: key.NewBinding(
		key.WithKeys("ctrl+o", "f2"),
		key.WithHelp("f2 / ctrl+o", "model selection"),
	),

	SwitchTheme: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("ctrl+t", "switch theme"),
	),
}

var helpEsc = key.NewBinding(
	key.WithKeys("?"),
	key.WithHelp("?", "toggle help"),
)

var returnKey = key.NewBinding(
	key.WithKeys("esc"),
	key.WithHelp("esc", "close"),
)

var logsKeyReturnKey = key.NewBinding(
	key.WithKeys("esc", "backspace", quitKey),
	key.WithHelp("esc/q", "go back"),
)

type appModel struct {
	width, height   int
	currentPage     page.PageID
	previousPage    page.PageID
	pages           map[page.PageID]util.Model
	loadedPages     map[page.PageID]bool
	status          core.StatusCmp
	app             *app.App
	selectedSession session.Session

	showPermissions bool
	permissions     dialog.PermissionDialogCmp

	showHelp bool
	help     dialog.HelpCmp

	showQuit bool
	quit     dialog.QuitDialog

	showSessionDialog bool
	sessionDialog     dialog.SessionDialog

	showAgents   bool
	agents       util.Model
	showSettings bool
	showConnect  bool
	connect      util.Model
	settings     util.Model

	showCommandDialog bool
	commandDialog     dialog.CommandDialog
	commands          []dialog.Command

	showModelDialog bool
	modelDialog     dialog.ModelDialog

	showInitDialog bool
	initDialog     dialog.InitDialogCmp

	showFilepicker bool
	filepicker     dialog.FilepickerCmp

	showThemeDialog bool
	themeDialog     dialog.ThemeDialog

	showMultiArgumentsDialog bool
	multiArgumentsDialog     dialog.MultiArgumentsDialogCmp

	isCompacting      bool
	compactingMessage string
}

func (a appModel) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	cmd := a.pages[a.currentPage].Init()
	a.loadedPages[a.currentPage] = true
	cmds = append(cmds, cmd)
	cmd = a.status.Init()
	cmds = append(cmds, cmd)
	cmd = a.quit.Init()
	cmds = append(cmds, cmd)
	cmd = a.help.Init()
	cmds = append(cmds, cmd)
	cmd = a.sessionDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.commandDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.modelDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.initDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.filepicker.Init()
	cmds = append(cmds, cmd)
	cmd = a.themeDialog.Init()
	cmds = append(cmds, cmd)

	// Check if we should show the init dialog
	cmds = append(cmds, func() tea.Msg {
		shouldShow, err := config.ShouldShowInitDialog()
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to check init status: " + err.Error(),
			}
		}
		return dialog.ShowInitDialogMsg{Show: shouldShow && a.app.CoderAgent.Model().ID != ""}
	})

	return tea.Batch(cmds...)
}

func (a appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	if batch, ok := msg.(MessageBatchMsg); ok {
		for _, event := range batch {
			updated, next := a.Update(event)
			a = updated.(appModel)
			cmds = append(cmds, next)
		}
		return a, tea.Batch(cmds...)
	}
	if a.showConnect {
		a.connect, cmd = a.connect.Update(msg)
		cmds = append(cmds, cmd)
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}
	if a.showAgents {
		var agentCmd tea.Cmd
		a.agents, agentCmd = a.agents.Update(msg)
		cmds = append(cmds, agentCmd)
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}
	if a.showModelDialog && !a.showQuit && !a.showPermissions && isInputMessage(msg) {
		updated, cmd := a.modelDialog.Update(msg)
		a.modelDialog = updated.(dialog.ModelDialog)
		return a, cmd
	}
	if a.isCompacting && isInputMessage(msg) {
		if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" {
			a.app.CoderAgent.Cancel(a.selectedSession.ID)
			a.compactingMessage = "Cancelling…"
			return a, a.syncPermissionPanel()
		}
		return a, nil
	}
	switch msg := msg.(type) {
	case dialog.CloseConnectMsg:
		a.showConnect = false
		return a, nil
	case dialog.ProviderConnectedMsg:
		if a.app.CoderAgent.IsBusy() {
			return a, util.ReportWarn("Wait for active requests before connecting a provider")
		}
		if err := config.RegisterConnection(msg.ID, msg.Connection); err != nil {
			return a, util.ReportError(err)
		}
		if err := config.SelectConnectedModel(models.ModelID(msg.ID + "/" + msg.Model.ID)); err != nil {
			return a, util.ReportError(err)
		}
		if err := a.app.CoderAgent.Reload(); err != nil {
			return a, util.ReportError(err)
		}
		a.showConnect = false
		a.modelDialog = dialog.NewModelDialogCmp()
		return a, tea.Batch(a.modelDialog.Init(), util.ReportInfo("Connected · "+msg.Model.Name))
	case tea.WindowSizeMsg:
		msg.Height -= 1 // Make space for the status bar
		a.width, a.height = msg.Width, msg.Height
		a.settings, _ = a.settings.Update(msg)
		a.agents, _ = a.agents.Update(msg)

		s, _ := a.status.Update(msg)
		a.status = s.(core.StatusCmp)
		a.pages[a.currentPage], cmd = a.pages[a.currentPage].Update(msg)
		cmds = append(cmds, cmd)

		prm, permCmd := a.permissions.Update(msg)
		a.permissions = prm.(dialog.PermissionDialogCmp)
		cmds = append(cmds, permCmd, a.syncPermissionPanel())

		help, helpCmd := a.help.Update(msg)
		a.help = help.(dialog.HelpCmp)
		cmds = append(cmds, helpCmd)

		session, sessionCmd := a.sessionDialog.Update(msg)
		a.sessionDialog = session.(dialog.SessionDialog)
		cmds = append(cmds, sessionCmd)

		command, commandCmd := a.commandDialog.Update(msg)
		a.commandDialog = command.(dialog.CommandDialog)
		cmds = append(cmds, commandCmd)

		filepicker, filepickerCmd := a.filepicker.Update(msg)
		a.filepicker = filepicker.(dialog.FilepickerCmp)
		cmds = append(cmds, filepickerCmd)

		a.initDialog.SetSize(msg.Width, msg.Height)

		if a.showMultiArgumentsDialog {
			a.multiArgumentsDialog.SetSize(msg.Width, msg.Height)
			args, argsCmd := a.multiArgumentsDialog.Update(msg)
			a.multiArgumentsDialog = args.(dialog.MultiArgumentsDialogCmp)
			cmds = append(cmds, argsCmd, a.multiArgumentsDialog.Init())
		}

		return a, tea.Batch(cmds...)
	// Status
	case util.InfoMsg:
		s, cmd := a.status.Update(msg)
		a.status = s.(core.StatusCmp)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)
	case pubsub.Event[logging.LogMessage]:
		if msg.Payload.Persist {
			switch msg.Payload.Level {
			case "error":
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeError,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})
				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)
			case "info":
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeInfo,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})
				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)

			case "warn":
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeWarn,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})

				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)
			default:
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeInfo,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})
				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)
			}
		}
	case util.ClearStatusMsg:
		s, _ := a.status.Update(msg)
		a.status = s.(core.StatusCmp)

	// Permission
	case pubsub.Event[permission.PermissionRequest]:
		a.showPermissions = true
		cmd := a.permissions.SetPermissions(msg.Payload)
		return a, tea.Batch(cmd, a.syncPermissionPanel())
	case dialog.PermissionResponseMsg:
		var cmd tea.Cmd
		switch msg.Action {
		case dialog.PermissionAllow:
			a.app.Permissions.Grant(msg.Permission)
		case dialog.PermissionAllowForSession:
			a.app.Permissions.GrantPersistant(msg.Permission)
		case dialog.PermissionDeny:
			a.app.Permissions.Deny(msg.Permission)
		}
		a.showPermissions = false
		return a, tea.Batch(cmd, a.syncPermissionPanel())

	case page.PageChangeMsg:
		return a, a.moveToPage(msg.ID)

	case dialog.CloseQuitMsg:
		a.showQuit = false
		return a, nil

	case dialog.CloseSessionDialogMsg:
		a.showSessionDialog = false
		return a, nil

	case dialog.CloseCommandDialogMsg:
		a.showCommandDialog = false
		return a, nil

	case compactStartFailedMsg:
		a.isCompacting = false
		return a, tea.Batch(a.syncPermissionPanel(), util.ReportError(msg.err))
	case startCompactSessionMsg:
		if a.app.CoderAgent.Model().ID == "" {
			return a, util.ReportWarn(agent.ErrNotConfigured.Error())
		}
		if a.selectedSession.ID == "" {
			return a, util.ReportWarn("No active session to compact")
		}
		if a.app.CoderAgent.IsSessionBusy(a.selectedSession.ID) {
			return a, util.ReportWarn("Wait for the current response before compacting")
		}
		settings := config.Get().Compaction
		opts := agent.CompactOptions{Method: settings.EffectiveMethod(), Mode: settings.EffectiveMode(), Focus: settings.Focus}
		id := a.selectedSession.ID
		a.isCompacting = true
		a.compactingMessage = "Compacting · " + opts.Method
		return a, tea.Batch(a.syncPermissionPanel(), func() tea.Msg {
			if err := a.app.CoderAgent.Summarize(context.Background(), id, opts); err != nil {
				return compactStartFailedMsg{err}
			}
			return nil
		})

	case pubsub.Event[agent.AgentEvent]:
		payload := msg.Payload
		if payload.SessionID != "" && payload.SessionID != a.selectedSession.ID {
			return a, tea.Batch(cmds...)
		}
		if payload.Error != nil {
			a.isCompacting = false
			return a, tea.Batch(a.syncPermissionPanel(), util.ReportError(payload.Error))
		}

		if payload.Type == agent.AgentEventTypeSummarize {
			a.compactingMessage = payload.Progress
		}
		if a.isCompacting {
			cmds = append(cmds, a.syncPermissionPanel())
		}

		if payload.Done && payload.Type == agent.AgentEventTypeSummarize {
			a.isCompacting = false
			return a, tea.Batch(a.syncPermissionPanel(), util.ReportInfo(payload.Progress))
		} else if payload.Done && payload.Type == agent.AgentEventTypeResponse && a.selectedSession.ID != "" {
			model := a.app.CoderAgent.Model()
			contextWindow := model.ContextWindow
			tokens := a.selectedSession.CompletionTokens + a.selectedSession.PromptTokens
			if contextWindow > 0 && tokens >= int64(float64(contextWindow)*float64(config.Get().Compaction.EffectiveThreshold())/100) && config.Get().AutoCompact {
				return a, util.CmdHandler(startCompactSessionMsg{})
			}
		}
		// Continue listening for events
		return a, tea.Batch(cmds...)

	case dialog.CloseThemeDialogMsg:
		a.showThemeDialog = false
		return a, nil

	case tea.BackgroundColorMsg:
		theme.SetDark(msg.IsDark())
		for id, model := range a.pages {
			updated, updateCmd := model.Update(dialog.ThemeChangedMsg{})
			a.pages[id] = updated
			cmds = append(cmds, updateCmd)
		}
		return a, tea.Batch(cmds...)

	case dialog.ThemeChangedMsg:
		a.pages[a.currentPage], cmd = a.pages[a.currentPage].Update(msg)
		a.showThemeDialog = false
		return a, tea.Batch(cmd, util.ReportInfo("Theme changed to: "+msg.ThemeName))

	case dialog.ConnectModelProviderMsg:
		a.showModelDialog = false
		return a, util.CmdHandler(chat.SlashCommandMsg("connect"))

	case dialog.CloseModelDialogMsg:
		a.showModelDialog = false
		return a, nil

	case dialog.ModelSelectedMsg:
		a.showModelDialog = false

		model, err := a.app.CoderAgent.Update(config.AgentCoder, msg.Model.ID)
		if err != nil {
			return a, util.ReportError(err)
		}

		if err := dialog.RecordRecentModel(model.ID); err != nil {
			return a, util.ReportWarn("Model changed, but recent models could not be saved: " + err.Error())
		}
		return a, util.ReportInfo(fmt.Sprintf("Model changed to %s", model.Name))

	case dialog.ShowInitDialogMsg:
		a.showInitDialog = msg.Show
		return a, nil

	case dialog.CloseInitDialogMsg:
		a.showInitDialog = false
		if msg.Initialize {
			// Run the initialization command
			for _, cmd := range a.commands {
				if cmd.ID == "init" {
					// Mark the project as initialized
					if err := config.MarkProjectInitialized(); err != nil {
						return a, util.ReportError(err)
					}
					return a, cmd.Handler(cmd)
				}
			}
		} else {
			// Mark the project as initialized without running the command
			if err := config.MarkProjectInitialized(); err != nil {
				return a, util.ReportError(err)
			}
		}
		return a, nil

	case chat.SessionClearedMsg:
		a.selectedSession = session.Session{}
	case chat.SessionSelectedMsg:
		a.selectedSession = msg
		a.sessionDialog.SetSelectedSession(msg.ID)

	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent && msg.Payload.ID == a.selectedSession.ID {
			a.selectedSession = msg.Payload
		}
	case dialog.SessionSelectedMsg:
		a.showSessionDialog = false
		if a.currentPage == page.ChatPage {
			return a, util.CmdHandler(chat.SessionSelectedMsg(msg.Session))
		}
		return a, nil

	case chat.SlashCommandMsg:
		switch string(msg) {
		case "new":
			a.selectedSession = session.Session{}
			return a, util.CmdHandler(chat.SessionClearedMsg{})
		case "models":
			return a, a.openModelDialog()
		case "themes":
			a.showThemeDialog = true
			return a, a.themeDialog.Init()
		case "agents":
			a.showAgents = true
			var load tea.Cmd
			a.agents, load = a.agents.Update(dialog.AgentParentMsg(a.selectedSession.ID))
			return a, load
		case "settings":
			a.showSettings = true
		case "reasoning":
			return a, a.cycleReasoning()
		case "connect":
			if a.app.CoderAgent.IsBusy() {
				return a, util.ReportWarn("Wait for active requests before connecting a provider")
			}
			a.connect = dialog.NewConnectCmp()
			a.connect, _ = a.connect.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
			a.showConnect = true

		case "sessions":
			return a, util.CmdHandler(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
		case "compact":
			return a, util.CmdHandler(startCompactSessionMsg{})
		case "help":
			a.showHelp = true
		}
		return a, nil
	case dialog.CloseAgentsMsg:
		a.showAgents = false
		return a, nil
	case dialog.CloseSettingsMsg:
		a.showSettings = false
		return a, nil
	case dialog.SettingsActionMsg:
		return a, util.CmdHandler(chat.SlashCommandMsg(msg))
	case dialog.CommandSelectedMsg:
		a.showCommandDialog = false
		// Execute the command handler if available
		if msg.Command.Handler != nil {
			return a, msg.Command.Handler(msg.Command)
		}
		return a, util.ReportInfo("Command selected: " + msg.Command.Title)

	case dialog.ShowMultiArgumentsDialogMsg:
		// Show multi-arguments dialog
		a.multiArgumentsDialog = dialog.NewMultiArgumentsDialogCmp(msg.CommandID, msg.Content, msg.ArgNames)
		a.showMultiArgumentsDialog = true
		return a, a.multiArgumentsDialog.Init()

	case dialog.CloseMultiArgumentsDialogMsg:
		// Close multi-arguments dialog
		a.showMultiArgumentsDialog = false

		// If submitted, replace all named arguments and run the command
		if msg.Submit {
			content := msg.Content

			// Replace each named argument with its value
			for name, value := range msg.Args {
				placeholder := "$" + name
				content = strings.ReplaceAll(content, placeholder, value)
			}

			// Execute the command with arguments
			return a, util.CmdHandler(dialog.CommandRunCustomMsg{
				Content: content,
				Args:    msg.Args,
			})
		}
		return a, nil

	case tea.MouseWheelMsg:
		if a.showSettings || a.showHelp {
			return a, nil
		}
	case tea.PasteMsg:
		if a.showSettings && !a.showModelDialog && !a.showThemeDialog && !a.showPermissions && !a.showQuit {
			a.settings, cmd = a.settings.Update(msg)
			return a, cmd
		}
		if a.showMultiArgumentsDialog {
			args, updateCmd := a.multiArgumentsDialog.Update(msg)
			a.multiArgumentsDialog = args.(dialog.MultiArgumentsDialogCmp)
			return a, updateCmd
		}

	case tea.KeyPressMsg:
		if a.showSettings && !a.showModelDialog && !a.showThemeDialog && !a.showPermissions && !a.showQuit {
			a.settings, cmd = a.settings.Update(msg)
			return a, cmd
		}
		// If multi-arguments dialog is open, let it handle the key press first
		if a.showMultiArgumentsDialog {
			args, cmd := a.multiArgumentsDialog.Update(msg)
			a.multiArgumentsDialog = args.(dialog.MultiArgumentsDialogCmp)
			return a, cmd
		}

		switch {

		case key.Matches(msg, keys.Quit):
			a.showQuit = !a.showQuit
			if a.showHelp {
				a.showHelp = false
			}
			if a.showSessionDialog {
				a.showSessionDialog = false
			}
			if a.showCommandDialog {
				a.showCommandDialog = false
			}
			if a.showFilepicker {
				a.showFilepicker = false
				a.filepicker.ToggleFilepicker(a.showFilepicker)
			}
			if a.showModelDialog {
				a.showModelDialog = false
			}
			if a.showMultiArgumentsDialog {
				a.showMultiArgumentsDialog = false
			}
			return a, nil
		case key.Matches(msg, keys.SwitchSession):
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showCommandDialog {
				// Load sessions and show the dialog
				sessions, err := a.app.Sessions.List(context.Background())
				if err != nil {
					return a, util.ReportError(err)
				}
				if len(sessions) == 0 {
					return a, util.ReportWarn("No sessions available")
				}
				a.sessionDialog.SetSessions(sessions)
				a.showSessionDialog = true
				return a, nil
			}
			return a, nil
		case key.Matches(msg, keys.Commands):
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showSessionDialog && !a.showThemeDialog && !a.showFilepicker {
				// Show commands dialog
				if len(a.commands) == 0 {
					return a, util.ReportWarn("No commands available")
				}
				a.commandDialog.SetCommands(a.commands)
				a.showCommandDialog = true
				return a, nil
			}
			return a, nil
		case key.Matches(msg, keys.Models):
			if a.showModelDialog {
				a.showModelDialog = false
				return a, nil
			}
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showSessionDialog && !a.showCommandDialog {
				return a, a.openModelDialog()
			}
			return a, nil
		case key.Matches(msg, keys.Reasoning):
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showSessionDialog && !a.showCommandDialog && !a.showModelDialog && !a.showThemeDialog && !a.showFilepicker {
				return a, a.cycleReasoning()
			}
			return a, nil
		case key.Matches(msg, keys.SwitchTheme):
			if !a.showQuit && !a.showPermissions && !a.showSessionDialog && !a.showCommandDialog {
				// Show theme switcher dialog
				a.showThemeDialog = true
				// Theme list is dynamically loaded by the dialog component
				return a, a.themeDialog.Init()
			}
			return a, nil
		case key.Matches(msg, returnKey) || key.Matches(msg):
			if msg.String() == quitKey {
				if a.currentPage == page.LogsPage {
					return a, a.moveToPage(page.ChatPage)
				}
			} else if !a.filepicker.IsCWDFocused() {
				if a.showQuit {
					a.showQuit = !a.showQuit
					return a, nil
				}
				if a.showHelp {
					a.showHelp = !a.showHelp
					return a, nil
				}
				if a.showInitDialog {
					a.showInitDialog = false
					// Mark the project as initialized without running the command
					if err := config.MarkProjectInitialized(); err != nil {
						return a, util.ReportError(err)
					}
					return a, nil
				}
				if a.showFilepicker {
					a.showFilepicker = false
					a.filepicker.ToggleFilepicker(a.showFilepicker)
					return a, nil
				}
				if a.currentPage == page.LogsPage {
					return a, a.moveToPage(page.ChatPage)
				}
			}
		case key.Matches(msg, keys.Logs):
			return a, a.moveToPage(page.LogsPage)
		case key.Matches(msg, keys.Help):
			if a.showQuit {
				return a, nil
			}
			a.showHelp = !a.showHelp
			return a, nil
		case key.Matches(msg, helpEsc):
			if a.app.CoderAgent.IsBusy() {
				if a.showQuit {
					return a, nil
				}
				a.showHelp = !a.showHelp
				return a, nil
			}
		case key.Matches(msg, keys.Filepicker):
			a.showFilepicker = !a.showFilepicker
			a.filepicker.ToggleFilepicker(a.showFilepicker)
			return a, nil
		}
	default:
		f, filepickerCmd := a.filepicker.Update(msg)
		a.filepicker = f.(dialog.FilepickerCmp)
		cmds = append(cmds, filepickerCmd)

	}

	if a.showFilepicker {
		f, filepickerCmd := a.filepicker.Update(msg)
		a.filepicker = f.(dialog.FilepickerCmp)
		cmds = append(cmds, filepickerCmd)
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showQuit {
		q, quitCmd := a.quit.Update(msg)
		a.quit = q.(dialog.QuitDialog)
		cmds = append(cmds, quitCmd)
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}
	if a.showPermissions {
		d, permissionsCmd := a.permissions.Update(msg)
		a.permissions = d.(dialog.PermissionDialogCmp)
		cmds = append(cmds, permissionsCmd, a.syncPermissionPanel())
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showSessionDialog {
		d, sessionCmd := a.sessionDialog.Update(msg)
		a.sessionDialog = d.(dialog.SessionDialog)
		cmds = append(cmds, sessionCmd)
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showCommandDialog {
		d, commandCmd := a.commandDialog.Update(msg)
		a.commandDialog = d.(dialog.CommandDialog)
		cmds = append(cmds, commandCmd)
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showModelDialog {
		d, modelCmd := a.modelDialog.Update(msg)
		a.modelDialog = d.(dialog.ModelDialog)
		cmds = append(cmds, modelCmd)
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showInitDialog {
		d, initCmd := a.initDialog.Update(msg)
		a.initDialog = d.(dialog.InitDialogCmp)
		cmds = append(cmds, initCmd)
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showThemeDialog {
		d, themeCmd := a.themeDialog.Update(msg)
		a.themeDialog = d.(dialog.ThemeDialog)
		cmds = append(cmds, themeCmd)
		// Only block key messages send all other messages down
		if isInputMessage(msg) {
			return a, tea.Batch(cmds...)
		}
	}

	s, _ := a.status.Update(msg)
	a.status = s.(core.StatusCmp)
	a.pages[a.currentPage], cmd = a.pages[a.currentPage].Update(msg)
	cmds = append(cmds, cmd)
	return a, tea.Batch(cmds...)
}

// syncPermissionPanel keeps the approval UI in the chat layout.
func (a *appModel) syncPermissionPanel() tea.Cmd {
	view := ""
	if a.showPermissions {
		width := a.width
		if page, ok := a.pages[page.ChatPage].(interface{ PermissionWidth() int }); ok {
			width = page.PermissionWidth()
		}
		updated, _ := a.permissions.Update(tea.WindowSizeMsg{Width: width, Height: a.height})
		a.permissions = updated.(dialog.PermissionDialogCmp)
		view = a.permissions.View()
	} else if a.isCompacting {
		t := theme.CurrentTheme()
		view = styles.BaseStyle().Foreground(t.Primary()).Padding(0, 1).Render(a.compactingMessage + "  ·  esc cancel")
	}
	pageModel, cmd := a.pages[page.ChatPage].Update(chat.PermissionPanelMsg(view))
	a.pages[page.ChatPage] = pageModel
	return cmd
}

// RegisterCommand adds a command to the command dialog
func (a *appModel) RegisterCommand(cmd dialog.Command) {
	a.commands = append(a.commands, cmd)
}

func (a *appModel) findCommand(id string) (dialog.Command, bool) {
	for _, cmd := range a.commands {
		if cmd.ID == id {
			return cmd, true
		}
	}
	return dialog.Command{}, false
}

func (a *appModel) moveToPage(pageID page.PageID) tea.Cmd {
	if a.app.CoderAgent.IsBusy() {
		// For now we don't move to any page if the agent is busy
		return util.ReportWarn("Agent is busy, please wait...")
	}

	var cmds []tea.Cmd
	if _, ok := a.loadedPages[pageID]; !ok {
		cmd := a.pages[pageID].Init()
		cmds = append(cmds, cmd)
		a.loadedPages[pageID] = true
	}
	a.previousPage = a.currentPage
	a.currentPage = pageID
	if sizable, ok := a.pages[a.currentPage].(layout.Sizeable); ok {
		cmd := sizable.SetSize(a.width, a.height)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (a appModel) render() string {
	components := []string{
		a.pages[a.currentPage].View(),
	}

	components = append(components, a.status.View())

	appView := lipgloss.JoinVertical(lipgloss.Top, components...)
	if a.showSettings {
		view := a.settings.View()
		appView = layout.PlaceOverlay(max(0, (a.width-lipgloss.Width(view))/2), max(0, (a.height-lipgloss.Height(view))/2), view, appView, false)
	}

	if a.showAgents {
		view := a.agents.View()
		appView = layout.PlaceOverlay(max(0, (a.width-lipgloss.Width(view))/2), max(0, (a.height-lipgloss.Height(view))/2), view, appView, false)
	}
	if a.showConnect {
		view := a.connect.View()
		appView = layout.PlaceOverlay(max(0, (a.width-lipgloss.Width(view))/2), max(0, (a.height-lipgloss.Height(view))/2), view, appView, false)
	}

	if a.showFilepicker {
		overlay := a.filepicker.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)

	}

	if a.showHelp {
		bindings := layout.KeyMapToSlice(keys)
		if p, ok := a.pages[a.currentPage].(layout.Bindings); ok {
			bindings = append(bindings, p.BindingKeys()...)
		}
		if a.showPermissions {
			bindings = append(bindings, a.permissions.BindingKeys()...)
		}
		if a.currentPage == page.LogsPage {
			bindings = append(bindings, logsKeyReturnKey)
		}
		if !a.app.CoderAgent.IsBusy() {
			bindings = append(bindings, helpEsc)
		}
		a.help.SetBindings(bindings)

		overlay := a.help.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showQuit {
		overlay := a.quit.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showSessionDialog {
		overlay := a.sessionDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showModelDialog {
		overlay := a.modelDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			false,
		)
	}

	if a.showCommandDialog {
		overlay := a.commandDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showInitDialog {
		overlay := a.initDialog.View()
		appView = layout.PlaceOverlay(
			a.width/2-lipgloss.Width(overlay)/2,
			a.height/2-lipgloss.Height(overlay)/2,
			overlay,
			appView,
			true,
		)
	}

	if a.showThemeDialog {
		overlay := a.themeDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showMultiArgumentsDialog {
		overlay := a.multiArgumentsDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	return appView
}

func (a appModel) cycleReasoning() tea.Cmd {
	if a.app.CoderAgent.IsBusy() {
		return util.ReportWarn("Wait for the current response before changing reasoning")
	}
	level, err := config.CycleReasoning()
	if err != nil {
		return util.ReportWarn(err.Error())
	}
	if err := a.app.CoderAgent.Reload(); err != nil {
		return util.ReportError(err)
	}
	return util.ReportInfo("Reasoning · " + level + " · Alt+R to change")
}

func New(app *app.App) tea.Model {
	startPage := page.ChatPage
	model := &appModel{
		currentPage:   startPage,
		loadedPages:   make(map[page.PageID]bool),
		status:        core.NewStatusCmp(app.LSPClients),
		help:          dialog.NewHelpCmp(),
		quit:          dialog.NewQuitCmp(),
		sessionDialog: dialog.NewSessionDialogCmp(),
		commandDialog: dialog.NewCommandDialogCmp(),
		settings:      dialog.NewSettingsCmp(),
		agents:        dialog.NewAgentsCmp(app),
		modelDialog:   dialog.NewModelDialogCmp(),
		permissions:   dialog.NewPermissionDialogCmp(),
		initDialog:    dialog.NewInitDialogCmp(),
		themeDialog:   dialog.NewThemeDialogCmp(),
		app:           app,
		commands:      []dialog.Command{},
		pages: map[page.PageID]util.Model{
			page.ChatPage: page.NewChatPage(app),
			page.LogsPage: page.NewLogsPage(),
		},
		filepicker: dialog.NewFilepickerCmp(app),
	}

	model.RegisterCommand(dialog.Command{
		ID:          "init",
		Title:       "Initialize Project",
		Description: "Create/Update the OwnCode.md memory file",
		Handler: func(cmd dialog.Command) tea.Cmd {
			prompt := `Please analyze this codebase and create a OwnCode.md file containing:
1. Build/lint/test commands - especially for running a single test
2. Code style guidelines including imports, formatting, types, naming conventions, error handling, etc.

The file you create will be given to agentic coding agents (such as yourself) that operate in this repository. Make it about 20 lines long.
If there's already a owncode.md, improve it.
If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include them.`
			return tea.Batch(
				util.CmdHandler(chat.SendMsg{
					Text: prompt,
				}),
			)
		},
	})

	model.RegisterCommand(dialog.Command{
		ID:          "compact",
		Title:       "Compact Session",
		Description: "Compact using Settings → Context preferences",
		Handler: func(cmd dialog.Command) tea.Cmd {
			return func() tea.Msg {
				return startCompactSessionMsg{}
			}
		},
	})
	// Load custom commands
	customCommands, err := dialog.LoadCustomCommands()
	if err != nil {
		logging.Warn("Failed to load custom commands", "error", err)
	} else {
		for _, cmd := range customCommands {
			model.RegisterCommand(cmd)
		}
	}

	return model
}

func (a appModel) View() tea.View {
	view := tea.NewView(a.render())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.BackgroundColor = theme.CurrentTheme().Background()
	view.ForegroundColor = theme.CurrentTheme().Text()
	return view
}

// isInputMessage keeps keyboard, paste, and wheel input inside the active dialog.
func isInputMessage(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.KeyReleaseMsg, tea.PasteMsg, tea.MouseWheelMsg, tea.MouseClickMsg, util.ScrollMsg:
		return true
	default:
		return false
	}
}

type compactStartFailedMsg struct{ err error }

func (a *appModel) openModelDialog() tea.Cmd {
	a.modelDialog = dialog.NewModelDialogCmp()
	cmd := a.modelDialog.Init()
	a.modelDialog.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
	a.showModelDialog = true
	return cmd
}
