package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/diff"
	"github.com/muratmirgun/owncode/internal/history"
	"github.com/muratmirgun/owncode/internal/pubsub"
	"github.com/muratmirgun/owncode/internal/session"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type sidebarCmp struct {
	width, height int
	session       session.Session
	history       history.Service
	modFiles      map[string]struct {
		additions int
		removals  int
	}
}

func (m *sidebarCmp) Init() tea.Cmd {
	if m.history != nil {
		ctx := context.Background()
		// Subscribe to file events
		filesCh := m.history.Subscribe(ctx)

		// Initialize the modified files map
		m.modFiles = make(map[string]struct {
			additions int
			removals  int
		})

		// Load initial files and calculate diffs
		m.loadModifiedFiles(ctx)

		// Return a command that will send file events to the Update method
		return func() tea.Msg {
			return <-filesCh
		}
	}
	return nil
}

func (m *sidebarCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SessionClearedMsg:
		m.session = session.Session{}
		clear(m.modFiles)
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			m.session = msg
			ctx := context.Background()
			m.loadModifiedFiles(ctx)
		}
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if m.session.ID == msg.Payload.ID {
				m.session = msg.Payload
			}
		}
	case pubsub.Event[history.File]:
		if msg.Payload.SessionID == m.session.ID {
			// Process the individual file change instead of reloading all files
			ctx := context.Background()
			m.processFileChanges(ctx, msg.Payload)

			// Return a command to continue receiving events
			return m, func() tea.Msg {
				ctx := context.Background()
				filesCh := m.history.Subscribe(ctx)
				return <-filesCh
			}
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
	rows := []string{heading("OwnCode"), line(filepath.Base(config.WorkingDirectory())), line(""), heading("Session"), line(title), line(""), heading("Language servers")}
	var names []string
	for name := range config.Get().LSP {
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
	return base.Width(m.width).Height(m.height).MaxHeight(m.height).Render(strings.Join(rows, "\n"))
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
			workingDir := config.WorkingDirectory()
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

func (m *sidebarCmp) processFileChanges(ctx context.Context, file history.File) {
	// Skip if this is the initial version (no changes to show)
	if file.Version == history.InitialVersion {
		return
	}

	// Find the initial version for this file
	initialVersion, err := m.findInitialVersion(ctx, file.Path)
	if err != nil || initialVersion.ID == "" {
		return
	}

	// Skip if content hasn't changed
	if initialVersion.Content == file.Content {
		// If this file was previously modified but now matches the initial version,
		// remove it from the modified files list
		displayPath := getDisplayPath(file.Path)
		delete(m.modFiles, displayPath)
		return
	}

	// Calculate diff between initial and latest version
	_, additions, removals := diff.GenerateDiff(initialVersion.Content, file.Content, file.Path)

	// Only add to modified files if there are changes
	if additions > 0 || removals > 0 {
		displayPath := getDisplayPath(file.Path)
		m.modFiles[displayPath] = struct {
			additions int
			removals  int
		}{
			additions: additions,
			removals:  removals,
		}
	} else {
		// If no changes, remove from modified files
		displayPath := getDisplayPath(file.Path)
		delete(m.modFiles, displayPath)
	}
}

// Helper function to find the initial version of a file
func (m *sidebarCmp) findInitialVersion(ctx context.Context, path string) (history.File, error) {
	// Get all versions of this file for the session
	fileVersions, err := m.history.ListBySession(ctx, m.session.ID)
	if err != nil {
		return history.File{}, err
	}

	// Find the initial version
	for _, v := range fileVersions {
		if v.Path == path && v.Version == history.InitialVersion {
			return v, nil
		}
	}

	return history.File{}, fmt.Errorf("initial version not found")
}

// Helper function to get the display path for a file
func getDisplayPath(path string) string {
	workingDir := config.WorkingDirectory()
	displayPath := strings.TrimPrefix(path, workingDir)
	return strings.TrimPrefix(displayPath, "/")
}
