package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/diff"
	"github.com/muratmirgun/owncode/internal/llm/tools"
	"github.com/muratmirgun/owncode/internal/permission"
	"github.com/muratmirgun/owncode/internal/tui/layout"
	"github.com/muratmirgun/owncode/internal/tui/styles"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type PermissionAction string

// Permission responses
const (
	PermissionAllow           PermissionAction = "allow"
	PermissionAllowForSession PermissionAction = "allow_session"
	PermissionDeny            PermissionAction = "deny"
)

// PermissionResponseMsg represents the user's response to a permission request
type PermissionResponseMsg struct {
	Permission permission.PermissionRequest
	Action     PermissionAction
}

// PermissionDialogCmp interface for permission dialog component
type PermissionDialogCmp interface {
	util.Model
	layout.Bindings
	SetPermissions(permission permission.PermissionRequest) tea.Cmd
}

type permissionsMapping struct {
	Left         key.Binding
	Right        key.Binding
	EnterSpace   key.Binding
	Allow        key.Binding
	AllowSession key.Binding
	Deny         key.Binding
	Tab          key.Binding
}

var permissionsKeys = permissionsMapping{
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "switch options"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "switch options"),
	),
	EnterSpace: key.NewBinding(
		key.WithKeys("enter", "space"),
		key.WithHelp("enter/space", "confirm"),
	),
	Allow: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "allow"),
	),
	AllowSession: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "allow for session"),
	),
	Deny: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "deny"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	),
}

// permissionDialogCmp is the implementation of PermissionDialog
type permissionDialogCmp struct {
	expanded        bool
	width           int
	height          int
	permission      permission.PermissionRequest
	windowSize      tea.WindowSizeMsg
	contentViewPort viewport.Model
	selectedOption  int // 0: Allow, 1: Allow for session, 2: Deny

	diffCache     map[string]string
	markdownCache map[string]string
}

func (p *permissionDialogCmp) Init() tea.Cmd {
	return p.contentViewPort.Init()
}

func (p *permissionDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.windowSize = msg
		cmd := p.SetSize()
		cmds = append(cmds, cmd)
		p.markdownCache = make(map[string]string)
		p.diffCache = make(map[string]string)
	case tea.KeyPressMsg:
		if msg.String() == "v" {
			p.expanded = !p.expanded
			return p, nil
		}
		switch {
		case key.Matches(msg, permissionsKeys.Right) || key.Matches(msg, permissionsKeys.Tab):
			p.selectedOption = (p.selectedOption + 1) % 3
			return p, nil
		case key.Matches(msg, permissionsKeys.Left):
			p.selectedOption = (p.selectedOption + 2) % 3
		case key.Matches(msg, permissionsKeys.EnterSpace):
			return p, p.selectCurrentOption()
		case key.Matches(msg, permissionsKeys.Allow):
			return p, util.CmdHandler(PermissionResponseMsg{Action: PermissionAllow, Permission: p.permission})
		case key.Matches(msg, permissionsKeys.AllowSession):
			return p, util.CmdHandler(PermissionResponseMsg{Action: PermissionAllowForSession, Permission: p.permission})
		case key.Matches(msg, permissionsKeys.Deny):
			return p, util.CmdHandler(PermissionResponseMsg{Action: PermissionDeny, Permission: p.permission})
		default:
			// Pass other keys to viewport
			viewPort, cmd := p.contentViewPort.Update(msg)
			p.contentViewPort = viewPort
			cmds = append(cmds, cmd)
		}
	}

	return p, tea.Batch(cmds...)
}

func (p *permissionDialogCmp) selectCurrentOption() tea.Cmd {
	var action PermissionAction

	switch p.selectedOption {
	case 0:
		action = PermissionAllow
	case 1:
		action = PermissionAllowForSession
	case 2:
		action = PermissionDeny
	}

	return util.CmdHandler(PermissionResponseMsg{Action: action, Permission: p.permission})
}

func (p *permissionDialogCmp) renderButtons() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	labels := []string{"Allow [a]", "Session [s]", "Deny [d]"}
	parts := make([]string, len(labels))
	for i, label := range labels {
		style := base.Foreground(t.TextMuted())
		if i == p.selectedOption {
			style = style.Foreground(t.Primary()).Bold(true)
			label = "› " + label
		}
		parts[i] = style.Render(label)
	}
	content := strings.Join(parts, base.Foreground(t.TextMuted()).Render("  /  "))
	if lipgloss.Width(content) > p.width {
		content = strings.Join(parts, "\n")
	}
	return content
}

func (p *permissionDialogCmp) renderHeader() string {
	t := theme.CurrentTheme()
	path := p.permission.Path
	switch params := p.permission.Params.(type) {
	case tools.EditPermissionsParams:
		path = params.FilePath
	case tools.WritePermissionsParams:
		path = params.FilePath
	}
	return styles.BaseStyle().Background(t.BackgroundSecondary()).Foreground(t.TextMuted()).
		Width(p.width).Render(ansi.Truncate(p.permission.ToolName+" · "+path, p.width, "…"))
}

func (p *permissionDialogCmp) renderBashContent() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle().Background(t.BackgroundSecondary())

	if pr, ok := p.permission.Params.(tools.BashPermissionsParams); ok {
		content := fmt.Sprintf("```bash\n%s\n```", pr.Command)

		// Use the cache for markdown rendering
		renderedContent := p.GetOrSetMarkdown(p.permission.ID, func() (string, error) {
			r := styles.GetMarkdownRenderer(max(1, p.width-2))
			s, err := r.Render(content)
			return styles.ForceReplaceBackgroundWithLipgloss(s, t.BackgroundSecondary()), err
		})

		finalContent := baseStyle.
			Width(p.contentViewPort.Width()).
			Render(renderedContent)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderEditContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width()))
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderPatchContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width()))
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderWriteContent() string {
	if pr, ok := p.permission.Params.(tools.WritePermissionsParams); ok {
		// Use the cache for diff rendering
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width()))
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderFetchContent() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle().Background(t.BackgroundSecondary())

	if pr, ok := p.permission.Params.(tools.FetchPermissionsParams); ok {
		content := fmt.Sprintf("```bash\n%s\n```", pr.URL)

		// Use the cache for markdown rendering
		renderedContent := p.GetOrSetMarkdown(p.permission.ID, func() (string, error) {
			r := styles.GetMarkdownRenderer(max(1, p.width-2))
			s, err := r.Render(content)
			return styles.ForceReplaceBackgroundWithLipgloss(s, t.BackgroundSecondary()), err
		})

		finalContent := baseStyle.
			Width(p.contentViewPort.Width()).
			Render(renderedContent)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderDefaultContent() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle().Background(t.BackgroundSecondary())

	content := p.permission.Description

	// Use the cache for markdown rendering
	renderedContent := p.GetOrSetMarkdown(p.permission.ID, func() (string, error) {
		r := styles.GetMarkdownRenderer(max(1, p.width-2))
		s, err := r.Render(content)
		return styles.ForceReplaceBackgroundWithLipgloss(s, t.BackgroundSecondary()), err
	})

	finalContent := baseStyle.
		Width(p.contentViewPort.Width()).
		Render(renderedContent)
	p.contentViewPort.SetContent(finalContent)

	if renderedContent == "" {
		return ""
	}

	return p.styleViewport()
}

func (p *permissionDialogCmp) styleViewport() string {
	t := theme.CurrentTheme()
	contentStyle := lipgloss.NewStyle().
		Background(t.BackgroundSecondary())

	return contentStyle.Render(p.contentViewPort.View())
}

func (p *permissionDialogCmp) render() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle().Background(t.BackgroundSecondary())
	title := base.Foreground(t.Primary()).Bold(true).Width(p.width).Render("Permission required")
	headerContent := p.renderHeader()
	buttons := p.renderButtons()
	p.contentViewPort.SetWidth(max(1, p.width))
	previewHeight := 2
	if p.expanded {
		previewHeight = max(2, p.height-4-lipgloss.Height(buttons))
	}
	p.contentViewPort.SetHeight(previewHeight)
	// Render content based on tool type
	var contentFinal string
	switch p.permission.ToolName {
	case tools.BashToolName:
		contentFinal = p.renderBashContent()
	case tools.EditToolName:
		contentFinal = p.renderEditContent()
	case tools.PatchToolName:
		contentFinal = p.renderPatchContent()
	case tools.WriteToolName:
		contentFinal = p.renderWriteContent()
	case tools.FetchToolName:
		contentFinal = p.renderFetchContent()
	default:
		contentFinal = p.renderDefaultContent()
	}

	hint := base.Foreground(t.TextMuted()).Render("←/→ choose · enter confirm · v details · ↑/↓ scroll")
	content := lipgloss.JoinVertical(lipgloss.Left, title, headerContent, contentFinal, buttons, hint)
	panel := base.Width(p.width+3).Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(t.Primary()).BorderBackground(t.BackgroundSecondary()).Render(content)
	return styles.Surface(panel, t.BackgroundSecondary())
}

func (p *permissionDialogCmp) View() string {
	return p.render()
}

func (p *permissionDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(permissionsKeys)
}

func (p *permissionDialogCmp) SetSize() tea.Cmd {
	if p.permission.ID == "" {
		return nil
	}
	p.width = max(1, p.windowSize.Width-3)
	p.height = max(6, min(12, p.windowSize.Height/3))

	return nil
}

func (p *permissionDialogCmp) SetPermissions(permission permission.PermissionRequest) tea.Cmd {
	p.permission = permission
	p.expanded = false
	p.contentViewPort.GotoTop()
	p.diffCache = make(map[string]string)
	p.markdownCache = make(map[string]string)
	return p.SetSize()
}

// Helper to get or set cached diff content
func (c *permissionDialogCmp) GetOrSetDiff(key string, generator func() (string, error)) string {
	if cached, ok := c.diffCache[key]; ok {
		return cached
	}

	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error formatting diff: %v", err)
	}

	c.diffCache[key] = content

	return content
}

// Helper to get or set cached markdown content
func (c *permissionDialogCmp) GetOrSetMarkdown(key string, generator func() (string, error)) string {
	if cached, ok := c.markdownCache[key]; ok {
		return cached
	}

	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error rendering markdown: %v", err)
	}

	c.markdownCache[key] = content

	return content
}

func NewPermissionDialogCmp() PermissionDialogCmp {
	// Create viewport for content
	contentViewport := viewport.New()

	return &permissionDialogCmp{
		contentViewPort: contentViewport,
		selectedOption:  0, // Default to "Allow"
		diffCache:       make(map[string]string),
		markdownCache:   make(map[string]string),
	}
}
