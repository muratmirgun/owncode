package dialog

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/auth"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// CloseConnectMsg returns to the previous screen without saving a draft.
type CloseConnectMsg struct{}

// ProviderConnectedMsg requests registration on the UI thread while the agent is idle.
type ProviderConnectedMsg struct {
	ID         string
	Connection auth.Connection
	Model      auth.Model
}

type connectResultMsg struct {
	owner      *connectCmp
	attempt    int
	connection auth.Connection
	err        error
}

type connectCmp struct {
	width, height          int
	selected               int
	stage                  string
	id, key, err, loginURL string
	connection             auth.Connection
	cancel                 context.CancelFunc
	attempt                int
}

// NewConnectCmp creates the provider setup screen.
func NewConnectCmp() util.Model     { return &connectCmp{stage: "providers"} }
func (c *connectCmp) Init() tea.Cmd { return nil }

func (c *connectCmp) reset() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.attempt++
	c.stage, c.key, c.err, c.loginURL = "providers", "", "", ""
	c.connection = auth.Connection{}
	c.selected = 0
}

func (c *connectCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width, c.height = msg.Width, msg.Height
	case connectResultMsg:
		if msg.owner != c || msg.attempt != c.attempt {
			return c, nil
		}
		if c.cancel != nil {
			c.cancel()
			c.cancel = nil
		}
		if msg.err != nil {
			if msg.connection.Token != nil && msg.connection.Token.Access != "" {
				c.connection = msg.connection
				c.stage, c.loginURL, c.err = "retry", "", msg.err.Error()
				return c, nil
			}
			c.reset()
			c.err = msg.err.Error()
			return c, nil
		}
		c.connection = msg.connection
		c.stage, c.selected = "models", 0
	case tea.PasteMsg:
		if c.stage == "key" {
			c.key = strings.TrimSpace(msg.Content)
		}
	case tea.KeyPressMsg:
		if msg.String() == "esc" || msg.String() == "ctrl+c" {
			closing := c.stage == "providers"
			c.reset()
			if closing {
				return c, util.CmdHandler(CloseConnectMsg{})
			}
			return c, nil
		}
		switch c.stage {
		case "providers":
			switch msg.String() {
			case "up", "down", "tab", "shift+tab":
				c.selected = 1 - c.selected
			case "enter":
				c.err = ""
				if c.selected == 0 {
					c.id = auth.ChatGPT
					return c, c.login()
				}
				c.id, c.stage, c.key = auth.Claude, "key", ""
			}
		case "key":
			switch msg.String() {
			case "enter":
				key := strings.TrimSpace(c.key)
				if key == "" || strings.ContainsAny(key, "\r\n\t ") {
					c.err = "Enter a valid API key."
					return c, nil
				}
				c.key, c.err, c.stage = "", "", "waiting"
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				c.cancel = cancel
				attempt := c.attempt
				return c, func() tea.Msg {
					connection := auth.Connection{Key: key}
					catalog, err := auth.Discover(ctx, auth.Claude, connection)
					connection.Models = catalog
					return connectResultMsg{owner: c, attempt: attempt, connection: connection, err: err}
				}
			case "backspace":
				chars := []rune(c.key)
				if len(chars) > 0 {
					c.key = string(chars[:len(chars)-1])
				}
			default:
				c.key += msg.Text
			}
		case "waiting":
			if msg.String() == "c" && c.loginURL != "" {
				url := c.loginURL
				return c, func() tea.Msg {
					if err := clipboard.WriteAll(url); err != nil {
						return util.ReportWarn("Cannot copy login URL")()
					}
					return util.ReportInfo("Login URL copied")()
				}
			}
		case "retry":
			if msg.String() == "enter" || msg.String() == "r" {
				return c, c.retryModels()
			}
		case "models":
			switch msg.String() {
			case "up":
				c.selected = max(0, c.selected-1)
			case "down":
				c.selected = min(len(c.connection.Models)-1, c.selected+1)
			case "enter":
				result := ProviderConnectedMsg{ID: c.id, Connection: c.connection, Model: c.connection.Models[c.selected]}
				c.reset()
				return c, util.CmdHandler(result)
			}
		}
	}
	return c, nil
}

func (c *connectCmp) login() tea.Cmd {
	login, err := auth.StartLogin()
	if err != nil {
		c.err = err.Error()
		return nil
	}
	c.stage, c.loginURL = "waiting", login.URL
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	c.cancel = cancel
	attempt := c.attempt
	return func() tea.Msg {
		defer login.Close()
		// The copy action remains available if the operating system cannot open a browser.
		launchCtx, stopLaunch := context.WithTimeout(ctx, 10*time.Second)
		_ = browserCommand(launchCtx, runtime.GOOS, login.URL).Run()
		stopLaunch()
		token, err := login.Wait(ctx)
		connection := auth.Connection{Token: &token}
		if err == nil {
			connection.Models, err = auth.Discover(ctx, auth.ChatGPT, connection)
		}
		return connectResultMsg{owner: c, attempt: attempt, connection: connection, err: err}
	}
}

// Nil output streams keep browser launch messages outside the terminal renderer.
func browserCommand(ctx context.Context, platform, url string) *exec.Cmd {
	switch platform {
	case "darwin":
		return exec.CommandContext(ctx, "open", url)
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return exec.CommandContext(ctx, "xdg-open", url)
	}
}

func (c *connectCmp) retryModels() tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	c.cancel = cancel
	c.stage, c.err = "waiting", ""
	connection, attempt, id := c.connection, c.attempt, c.id
	return func() tea.Msg {
		catalog, err := auth.Discover(ctx, id, connection)
		connection.Models = catalog
		return connectResultMsg{owner: c, attempt: attempt, connection: connection, err: err}
	}
}

func (c *connectCmp) View() string {
	t := theme.CurrentTheme()
	width := max(16, min(76, c.width-4))
	inner := max(1, width-6)
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	line := func(text string) string { return base.Width(inner).Render(ansi.Truncate(text, inner, "…")) }
	rows := []string{base.Foreground(t.Text()).Bold(true).Render("Connect a provider"), ""}
	help := "↑/↓ choose · enter continue · esc close"
	switch c.stage {
	case "providers":
		rows = append(rows, line("Choose how OwnCode accesses your models."), "")
		for i, item := range []struct{ id, title, detail string }{
			{auth.ChatGPT, "OpenAI · ChatGPT account", "Sign in through your browser"},
			{auth.Claude, "Claude · API key", "Connect your Anthropic Console key"},
		} {
			style := base.Foreground(t.Text())
			prefix := "  "
			if i == c.selected {
				style = style.Foreground(t.Primary()).Bold(true)
				prefix = "› "
			}
			status := "Not connected"
			if cfg := config.Get(); cfg != nil {
				if p, ok := cfg.Providers[models.ModelProvider(item.id)]; ok && p.HasCredentials() && !p.Disabled {
					status = "Configured"
				}
			}
			rows = append(rows, style.Width(inner).Render(ansi.Truncate(prefix+item.title, inner, "…")), line("  "+item.detail), line("  "+status), "")
		}
	case "key":
		rows = append(rows, line("Claude · Anthropic API key"), line("Create a key at platform.claude.com/settings/keys"), "",
			line("Key  "+strings.Repeat("•", min(48, len(c.key)))+"▏"), "", line("OwnCode checks access before saving the key."))
		help = "enter check key · esc back"
	case "waiting":
		text := "Checking your key and loading models…"
		if c.connection.Token != nil {
			text = "Loading ChatGPT models…"
		}
		if c.loginURL != "" {
			text = "Complete ChatGPT login in your browser."
		}
		rows = append(rows, line(text), "", line("Credentials stay in your private user settings."))
		help = "esc cancel"
		if c.loginURL != "" {
			rows = append(rows, "", line("Browser did not open? Copy the login link."))
			help = "c copy login URL · esc cancel"
		}
	case "retry":
		rows = append(rows, line("ChatGPT login completed."), "", line("The model list could not be loaded."), line("Retry without signing in again, or go back."))
		help = "enter retry models · esc back"
	case "models":
		rows = append(rows, line("Choose a model for chat, tasks, titles, and summaries."), "")
		count := max(1, min(10, c.height-13))
		start := max(0, c.selected-count+1)
		for i := start; i < min(len(c.connection.Models), start+count); i++ {
			style := base.Foreground(t.Text())
			prefix := "  "
			if i == c.selected {
				style = style.Foreground(t.Primary()).Bold(true)
				prefix = "› "
			}
			rows = append(rows, style.Width(inner).Render(ansi.Truncate(prefix+c.connection.Models[i].Name, inner, "…")))
		}
		rows = append(rows, "", line(fmt.Sprintf("%d models available", len(c.connection.Models))))
		help = "↑/↓ choose · enter connect · esc cancel"
	}
	if c.err != "" {
		rows = append(rows, "", base.Foreground(t.Error()).Width(inner).Render(c.err))
	}
	rows = append(rows, "", line(help))
	return styles.Surface(base.Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(t.BorderFocused()).Render(strings.Join(rows, "\n")), t.BackgroundSecondary())
}
