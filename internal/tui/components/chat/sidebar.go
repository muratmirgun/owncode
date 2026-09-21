package chat

import (
	"context"
	"fmt"
	"github.com/muratmirgun/owncode/internal/version"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/diff"
	"github.com/muratmirgun/owncode/internal/history"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type sidebarCmp struct {
	width, height            int
	filesLoading, filesDirty bool
	generation               uint64
	workingDir               string
	session                  session.Session
	history                  history.Service
	modFiles                 map[string]struct {
		additions int
		removals  int
	}
}

type sidebarLoadedMsg struct {
	owner      *sidebarCmp
	sessionID  string
	generation uint64
	files      map[string]struct {
		additions int
		removals  int
	}
}

func (m *sidebarCmp) Init() tea.Cmd { return m.refreshFiles() }

func (m *sidebarCmp) refreshFiles() tea.Cmd {
	if m.history == nil || m.session.ID == "" {
		return nil
	}
	m.filesDirty = true
	if m.filesLoading {
		return nil
	}
	m.filesLoading, m.filesDirty = true, false
	// Only this value copy is touched by the background command.
	snapshot := *m
	snapshot.workingDir = config.WorkingDirectory()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		snapshot.loadModifiedFiles(ctx)
		return sidebarLoadedMsg{owner: m, sessionID: snapshot.session.ID, generation: snapshot.generation, files: snapshot.modFiles}
	}
}

func (m *sidebarCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sidebarLoadedMsg:
		if msg.owner != m {
			return m, nil
		}
		m.filesLoading = false
		if msg.sessionID == m.session.ID && msg.generation == m.generation {
			m.modFiles = msg.files
		}
		if m.filesDirty {
			return m, m.refreshFiles()
		}
	case SessionClearedMsg:
		m.generation++
		m.session = session.Session{}
		m.modFiles = nil
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			m.generation++
			m.session = msg
			m.modFiles = nil
			return m, m.refreshFiles()
		}
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent && m.session.ID == msg.Payload.ID {
			m.session = msg.Payload
		}
	case pubsub.Event[history.File]:
		if msg.Payload.SessionID == m.session.ID {
			return m, m.refreshFiles()
		}
	}
	return m, nil
}

func (m *sidebarCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	line := func(text string) string {
		return base.Foreground(t.TextMuted()).Width(m.width).Render(ansi.Truncate(text, max(1, m.width), "…"))
	}
	heading := func(text string) string {
		return base.Foreground(t.Text()).Bold(true).Width(m.width).Render(ansi.Truncate(text, max(1, m.width), "…"))
	}
	title := m.session.Title
	if title == "" {
		title = "New chat"
	}
	rows := []string{heading(title)}
	if m.session.ID != "" {
		rows = append(rows, line(m.session.ID))
	}
	rows = append(rows, line(""), heading("Context"))
	cfg := config.Get()
	model := models.SupportedModels[cfg.Agents[config.AgentCoder].Model]
	used := max(int64(0), m.session.PromptTokens+m.session.CompletionTokens)
	if model.ContextWindow > 0 {
		percent := int(float64(used) * 100 / float64(model.ContextWindow))
		rows = append(rows, line(fmt.Sprintf("%s / %s tokens", groupDigits(used), groupDigits(model.ContextWindow))), line(fmt.Sprintf("%d%% used", percent)))
		barWidth := max(1, min(28, m.width))
		filled := min(barWidth, max(0, int(float64(used)*float64(barWidth)/float64(model.ContextWindow))))
		if used > 0 && filled == 0 {
			filled = 1
		}
		color := t.Primary()
		if percent >= cfg.Compaction.EffectiveThreshold() {
			color = t.Warning()
		}
		bar := base.Foreground(color).Render(strings.Repeat("█", filled)) + base.Foreground(t.BorderNormal()).Render(strings.Repeat("░", barWidth-filled))
		rows = append(rows, base.Width(m.width).Render(bar))
	} else {
		rows = append(rows, line("Model not configured"))
	}
	rows = append(rows, line(fmt.Sprintf("$%.2f spent", m.session.Cost)))
	mode := cfg.Compaction.EffectiveMethod()
	if cfg.AutoCompact {
		mode += fmt.Sprintf(" · auto %d%%", cfg.Compaction.EffectiveThreshold())
	} else {
		mode += " · manual"
	}
	rows = append(rows, line(mode), line(""), heading("MCP"))
	var servers []string
	for name := range cfg.MCPServers {
		servers = append(servers, name)
	}
	sort.Strings(servers)
	for _, name := range servers {
		rows = append(rows, line("• "+name+" · on demand"))
	}
	if len(servers) == 0 {
		rows = append(rows, line("None configured"))
	}
	rows = append(rows, line(""), heading("LSP"))
	var names []string
	for name, server := range cfg.LSP {
		if server.Disabled {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		rows = append(rows, line("• "+name))
	}
	if len(names) == 0 {
		rows = append(rows, line("None configured"))
	}
	rows = append(rows, line(""), heading("Modified files"))
	var paths []string
	for path := range m.modFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		stats := m.modFiles[path]
		rows = append(rows, line(fmt.Sprintf("%s  +%d −%d", path, stats.additions, stats.removals)))
	}
	if len(paths) == 0 {
		rows = append(rows, line("No modified files"))
	}
	cwd := config.WorkingDirectory()
	if home, err := os.UserHomeDir(); err == nil {
		if cwd == home {
			cwd = "~"
		} else if strings.HasPrefix(cwd, home+string(filepath.Separator)) {
			cwd = "~" + strings.TrimPrefix(cwd, home)
		}
	}
	build := version.Version
	if build == "unknown" {
		build = "dev"
	}
	footer := []string{line(cwd), line(""), heading("OwnCode " + build)}
	available := max(0, m.height-len(footer)-1)
	if len(rows) > available {
		rows = rows[:available]
	}
	for len(rows) < max(0, m.height-len(footer)) {
		rows = append(rows, line(""))
	}
	rows = append(rows, footer...)
	return styles.Surface(base.Width(m.width).Height(m.height).MaxHeight(m.height).Render(strings.Join(rows, "\n")), t.BackgroundSecondary())
}

func (m *sidebarCmp) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	return nil
}

func (m *sidebarCmp) GetSize() (int, int) {
	return m.width, m.height
}

func NewSidebarCmp(session session.Session, history history.Service) util.Model {
	return &sidebarCmp{
		session: session,
		history: history,
	}
}

func (m *sidebarCmp) loadModifiedFiles(ctx context.Context) {
	if m.history == nil || m.session.ID == "" {
		return
	}

	// Get all latest files for this session
	latestFiles, err := m.history.ListLatestSessionFiles(ctx, m.session.ID)
	if err != nil {
		return
	}

	// Get all files for this session (to find initial versions)
	allFiles, err := m.history.ListBySession(ctx, m.session.ID)
	if err != nil {
		return
	}

	// Clear the existing map to rebuild it
	m.modFiles = make(map[string]struct {
		additions int
		removals  int
	})

	// Process each latest file
	for _, file := range latestFiles {
		// Skip if this is the initial version (no changes to show)
		if file.Version == history.InitialVersion {
			continue
		}

		// Find the initial version for this specific file
		var initialVersion history.File
		for _, v := range allFiles {
			if v.Path == file.Path && v.Version == history.InitialVersion {
				initialVersion = v
				break
			}
		}

		// Skip if we can't find the initial version
		if initialVersion.ID == "" {
			continue
		}
		if initialVersion.Content == file.Content {
			continue
		}

		// Calculate diff between initial and latest version
		_, additions, removals := diff.GenerateDiff(initialVersion.Content, file.Content, file.Path)

		// Only add to modified files if there are changes
		if additions > 0 || removals > 0 {
			// Remove working directory prefix from file path
			displayPath := file.Path
			workingDir := m.workingDir
			displayPath = strings.TrimPrefix(displayPath, workingDir)
			displayPath = strings.TrimPrefix(displayPath, "/")

			m.modFiles[displayPath] = struct {
				additions int
				removals  int
			}{
				additions: additions,
				removals:  removals,
			}
		}
	}
}

func groupDigits(value int64) string {
	text := fmt.Sprint(value)
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return text
}
