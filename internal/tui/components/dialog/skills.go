package dialog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/muratmirgun/owncode/internal/skills"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// CloseSkillsMsg closes skill management.
type CloseSkillsMsg struct{}

// SkillsChangedMsg refreshes the agent's catalog after a successful change.
type SkillsChangedMsg struct{}

// SkillUseMsg inserts selected instructions into the composer without sending them.
type SkillUseMsg struct{ Content string }
type skillsResultMsg struct {
	id       int
	entries  []skills.Entry
	results  []skills.Result
	proposal *skills.Proposal
	detail   string
	changed  bool
	err      error
}

type skillsCmp struct {
	workdir                                     string
	roots                                       []skills.Root
	width, height, tab, selected, scroll, id    int
	input                                       textinput.Model
	entries                                     []skills.Entry
	results                                     []skills.Result
	proposal                                    *skills.Proposal
	expected, installRoot, mode, notice, detail string
	cancel                                      context.CancelFunc
	busy                                        bool
	committing                                  bool
}

// NewSkillsCmp creates an asynchronous skills browser and installer.
func NewSkillsCmp(workdir string) util.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "Search installed skills…"
	input.CharLimit = 512
	return &skillsCmp{workdir: workdir, roots: skills.Roots(workdir), width: 80, height: 30, input: input}
}
func (s *skillsCmp) operation(run func(context.Context) skillsResultMsg) tea.Cmd {
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	s.cancel = cancel
	s.id++
	id := s.id
	s.busy = true
	s.notice = "Working…"
	return func() tea.Msg { defer cancel(); result := run(ctx); result.id = id; return result }
}
func (s *skillsCmp) refresh() tea.Cmd {
	roots := append([]skills.Root(nil), s.roots...)
	return s.operation(func(ctx context.Context) skillsResultMsg {
		entries, problems := skills.Discover(ctx, roots)
		notice := ""
		if len(problems) > 0 {
			notice = strings.Join(problems, "\n")
		}
		return skillsResultMsg{entries: entries, detail: notice}
	})
}
func (s *skillsCmp) Init() tea.Cmd { return tea.Batch(s.input.Focus(), s.refresh()) }
func (s *skillsCmp) filtered() []skills.Entry {
	result := []skills.Entry{}
	query := strings.ToLower(s.input.Value())
	for _, entry := range s.entries {
		if strings.Contains(strings.ToLower(entry.Name+" "+entry.Description), query) {
			result = append(result, entry)
		}
	}
	return result
}
func (s *skillsCmp) current() (skills.Entry, bool) {
	entries := s.filtered()
	if s.selected < 0 || s.selected >= len(entries) {
		return skills.Entry{}, false
	}
	return entries[s.selected], true
}
func (s *skillsCmp) prepare(source, name string) tea.Cmd {
	return s.operation(func(ctx context.Context) skillsResultMsg {
		proposal, err := skills.Prepare(ctx, source, name)
		return skillsResultMsg{proposal: &proposal, err: err}
	})
}
func (s *skillsCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		s.input.SetWidth(max(1, min(82, s.width-8)))
	case skillsResultMsg:
		if msg.id != s.id {
			return s, nil
		}
		s.busy = false
		s.committing = false
		s.cancel = nil
		if msg.err != nil {
			s.notice = msg.err.Error()
			return s, nil
		}
		if msg.proposal != nil {
			s.proposal = msg.proposal
			s.mode = "confirm"
			s.scroll = 0
			s.notice = "Inspect the source and files. Enter installs; Esc cancels."
			return s, nil
		}
		if msg.entries != nil {
			s.entries = msg.entries
			s.notice = msg.detail
		}
		if msg.results != nil {
			s.results = msg.results
			s.selected = 0
			s.notice = fmt.Sprintf("%d catalog results", len(msg.results))
			if msg.detail != "" {
				s.notice += " · " + msg.detail
			}
		}
		if msg.changed {
			s.mode = ""
			s.input.SetValue("")
			s.detail = ""
			return s, tea.Batch(s.refresh(), util.CmdHandler(SkillsChangedMsg{}))
		}
		return s, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if s.busy {
			if key == "esc" && !s.committing {
				s.cancel()
				s.id++
				s.busy = false
				s.notice = "Canceled"
			}
			return s, nil
		}
		if key == "esc" {
			if s.mode != "" {
				s.mode = ""
				s.detail = ""
				s.proposal = nil
				s.input.SetValue("")
				return s, nil
			}
			return s, util.CmdHandler(CloseSkillsMsg{})
		}
		if s.mode == "confirm" {
			if key == "enter" {
				s.committing = true
				proposal, root, expected := *s.proposal, s.installRoot, s.expected
				return s, s.operation(func(ctx context.Context) skillsResultMsg {
					if err := ctx.Err(); err != nil {
						return skillsResultMsg{err: err}
					}
					err := skills.Apply(root, proposal, expected)
					return skillsResultMsg{changed: err == nil, err: err}
				})
			}
			if key == "up" {
				s.scroll = max(0, s.scroll-1)
			}
			if key == "down" {
				s.scroll++
			}
			return s, nil
		}
		if s.mode == "remove" {
			if key == "enter" {
				entry, ok := s.current()
				if !ok {
					return s, nil
				}
				s.committing = true
				return s, s.operation(func(ctx context.Context) skillsResultMsg {
					if err := ctx.Err(); err != nil {
						return skillsResultMsg{err: err}
					}
					err := skills.Remove(entry)
					return skillsResultMsg{changed: err == nil, err: err}
				})
			}
			return s, nil
		}
		if s.mode == "detail" {
			if key == "up" {
				s.scroll = max(0, s.scroll-1)
			}
			if key == "down" {
				s.scroll++
			}
			if key == "a" {
				entry, ok := s.current()
				if !ok {
					return s, nil
				}
				if entry.UserInvocable != nil && !*entry.UserInvocable {
					s.notice = "This skill does not allow explicit invocation"
					return s, nil
				}
				content, err := skills.Load(entry, "")
				if err != nil {
					s.notice = err.Error()
					return s, nil
				}
				return s, util.CmdHandler(SkillUseMsg{Content: content})
			}
			return s, nil
		}
		if s.mode == "source" {
			if key == "enter" {
				s.expected = ""
				return s, s.prepare(s.input.Value(), "")
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}
		switch key {
		case "tab":
			s.tab = 1 - s.tab
			s.selected = 0
			s.input.SetValue("")
			s.notice = ""
			return s, nil
		case "up":
			s.selected = max(0, s.selected-1)
			return s, nil
		case "down":
			count := len(s.results)
			if s.tab == 0 {
				count = len(s.filtered())
			}
			s.selected = min(max(0, count-1), s.selected+1)
			return s, nil
		case "ctrl+i", "f2":
			s.mode = "source"
			s.installRoot = s.roots[0].Path
			s.input.SetValue("")
			s.notice = "Install in project: owner/repo#path/to/skill, HTTPS URL, or local path"
			return s, nil
		case "f3":
			s.mode = "source"
			s.installRoot = s.roots[2].Path
			s.input.SetValue("")
			s.notice = "Install for user: owner/repo#path/to/skill, HTTPS URL, or local path"
			return s, nil
		case "ctrl+r":
			return s, s.refresh()
		case "ctrl+e":
			entry, ok := s.current()
			if s.tab != 0 || !ok {
				return s, nil
			}
			s.committing = true
			return s, s.operation(func(ctx context.Context) skillsResultMsg {
				if err := ctx.Err(); err != nil {
					return skillsResultMsg{err: err}
				}
				err := skills.SetEnabled(entry, entry.Disabled)
				return skillsResultMsg{changed: err == nil, err: err}
			})
		case "ctrl+b":
			entry, ok := s.current()
			if s.tab != 0 || !ok {
				return s, nil
			}
			manifest, err := skills.Installed(entry)
			if err != nil {
				s.notice = err.Error()
				return s, nil
			}
			s.installRoot = filepath.Dir(filepath.Dir(entry.Path))
			s.expected = manifest.Digest
			return s, s.operation(func(ctx context.Context) skillsResultMsg {
				proposal, err := skills.RollbackProposal(ctx, entry)
				return skillsResultMsg{proposal: &proposal, err: err}
			})
		case "ctrl+u":
			entry, ok := s.current()
			if s.tab != 0 || !ok {
				return s, nil
			}
			manifest, err := skills.Installed(entry)
			if err != nil {
				s.notice = "Only managed skills support updates"
				return s, nil
			}
			s.installRoot = filepath.Dir(filepath.Dir(entry.Path))
			s.expected = manifest.Digest
			return s, s.prepare(manifest.Source+"#"+manifest.Subdir, entry.Name)
		case "ctrl+d":
			if _, ok := s.current(); s.tab == 0 && ok {
				s.mode = "remove"
				s.notice = "Remove this managed skill? Enter confirms; Esc cancels."
			}
			return s, nil
		case "enter":
			if s.tab == 1 {
				query := s.input.Value()
				token := os.Getenv("OWNCODE_SKILLS_TOKEN")
				return s, s.operation(func(ctx context.Context) skillsResultMsg {
					results, stale, err := skills.SearchCached(ctx, "", token, query)
					notice := ""
					if stale {
						notice = "Cached results; catalog unavailable"
					}
					if results == nil {
						results = []skills.Result{}
					}
					return skillsResultMsg{results: results, detail: notice, err: err}
				})
			}
			entry, ok := s.current()
			if !ok {
				return s, nil
			}
			s.mode = "detail"
			s.scroll = 0
			s.detail = entry.Description + "\n\nSource: " + entry.Path + "\nSHA256: " + entry.Hash + "\n"
			for _, path := range entry.Shadowed {
				s.detail += "Overrides: " + path + "\n"
			}
			content, err := skills.Load(entry, "")
			if err != nil {
				s.detail += "\n" + err.Error()
			} else {
				s.detail += "\n" + content
			}
			s.notice = "↑↓ scroll · a insert instructions · esc back"
			return s, nil
		case "ctrl+a":
			if s.tab == 1 && s.selected < len(s.results) {
				item := s.results[s.selected]
				source := item.InstallURL
				if source == "" {
					source = item.Source
				}
				s.installRoot = s.roots[0].Path
				s.expected = ""
				return s, s.prepare(source, item.Slug)
			}
			return s, nil
		}
	}
	before := s.input.Value()
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	if before != s.input.Value() {
		s.selected = 0
	}
	return s, cmd
}
func (s *skillsCmp) View() string {
	t := theme.CurrentTheme()
	width := max(1, min(94, s.width-2))
	inner := max(1, width-4)
	height := max(1, min(20, s.height-12))
	base := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.Text())
	muted := base.Foreground(t.TextMuted())
	line := func(text string) string { return base.Width(inner).Render(ansi.Truncate(text, inner, "…")) }
	inputStyles := s.input.Styles()
	inputStyles.Focused.Text = base
	inputStyles.Focused.Prompt = base
	inputStyles.Focused.Placeholder = muted
	s.input.SetStyles(inputStyles)
	title := "Skills · Installed"
	if s.tab == 1 {
		title = "Skills · Discover on skills.sh"
	}
	if s.mode == "source" {
		title = "Install skill"
	}
	lines := []string{line(base.Bold(true).Render(title)), line(""), line(s.input.View()), line("")}
	rows := []string{}
	switch s.mode {
	case "confirm":
		p := s.proposal
		rows = append(rows, "Install "+p.Entry.Name, "Source: "+p.Manifest.Source, "Revision: "+p.Manifest.Revision, "Destination: "+filepath.Join(s.installRoot, p.Entry.Name), "")
		names := []string{}
		for name := range p.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			rows = append(rows, fmt.Sprintf("  %s · %d bytes", name, len(p.Files[name])))
		}
		rows = append(rows, "", "Instructions:")
		_, body, _ := skills.Parse(p.Files["SKILL.md"])
		rows = append(rows, strings.Split(ansi.Wrap(ansi.Strip(body), inner, ""), "\n")...)
	case "detail":
		rows = strings.Split(ansi.Wrap(ansi.Strip(s.detail), inner, ""), "\n")
	case "remove":
		entry, _ := s.current()
		rows = []string{"Remove " + entry.Name, entry.Path, "Locally modified files will not be removed."}
	default:
		if s.tab == 0 {
			for i, entry := range s.filtered() {
				state := entry.Scope
				if entry.Disabled {
					state += " · disabled"
				}
				text := "  " + entry.Name + " · " + state
				if i == s.selected {
					text = base.Foreground(t.Primary()).Bold(true).Render("› " + entry.Name + " · " + state)
				}
				rows = append(rows, text)
			}
		} else {
			for i, item := range s.results {
				text := "  " + item.Name + " · " + item.Source
				if i == s.selected {
					text = base.Foreground(t.Primary()).Bold(true).Render("› " + item.Name + " · " + item.Source)
				}
				rows = append(rows, text)
			}
		}
	}
	if len(rows) == 0 {
		rows = []string{"No skills to show. F2 installs a project skill; F3 installs a user skill."}
	}
	start := max(0, s.selected-height+1)
	if s.mode != "" {
		start = min(s.scroll, max(0, len(rows)-height))
	}
	start = min(start, max(0, len(rows)-height))
	for _, row := range rows[start:min(len(rows), start+height)] {
		lines = append(lines, line(row))
	}
	notice := s.notice
	if s.busy {
		notice = "Working… · esc cancel"
		if s.committing {
			notice = "Saving skill changes…"
		}
	}
	for _, row := range strings.Split(ansi.Wrap(ansi.Strip(notice), inner, ""), "\n") {
		lines = append(lines, line(muted.Render(row)))
		if len(lines) >= height+8 {
			break
		}
	}
	hint := "tab installed/discover · enter inspect/search · ctrl+a install result"
	if s.tab == 0 {
		hint = "F2 project · F3 user · ctrl+e enable · ctrl+u update · ctrl+d remove"
	}
	lines = append(lines, line(""), line(muted.Render(hint)), line(muted.Render("ctrl+b rollback · ctrl+r refresh · esc back")))
	view := base.Width(width).Padding(1, 2).Render(strings.Join(lines, "\n"))
	return lipgloss.NewStyle().MaxWidth(max(1, s.width)).MaxHeight(max(1, s.height)).Render(styles.Surface(view, t.BackgroundSecondary()))
}
