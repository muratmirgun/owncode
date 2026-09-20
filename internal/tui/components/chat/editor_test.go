package chat

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/tui/util"
	"github.com/stretchr/testify/require"
)

type unconfiguredAgent struct{ agent.Service }

func (unconfiguredAgent) Model() models.Model { return models.Model{} }

func TestSendWithoutModelPreservesDraft(t *testing.T) {
	t.Parallel()
	editor := &editorCmp{
		app:         &app.App{CoderAgent: unconfiguredAgent{}},
		textarea:    textarea.New(),
		attachments: []message.Attachment{{FileName: "draft.txt"}},
	}
	editor.textarea.SetValue("keep this draft")
	cmd := editor.send()
	require.NotNil(t, cmd)
	warning, ok := cmd().(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeWarn, warning.Type)
	require.Equal(t, "keep this draft", editor.textarea.Value())
	require.Len(t, editor.attachments, 1)
}

func TestEditorHeightFollowsDraft(t *testing.T) {
	editor := &editorCmp{textarea: textarea.New()}
	for _, test := range []struct {
		text          string
		width, height int
	}{
		{"", 40, 3},
		{"hello", 40, 3},
		{"one\ntwo\nthree", 40, 5},
		{strings.Repeat("x", 50), 23, 5},
		{strings.Repeat("line\n", 20), 40, 10},
		{"", 40, 3},
	} {
		editor.textarea.SetValue(test.text)
		require.Equal(t, test.height, editor.PreferredHeight(test.width))
	}
	editor.home = true
	require.Equal(t, 4, editor.PreferredHeight(40))
}

func TestEditorAcceptsPasteWithoutSending(t *testing.T) {
	editor := &editorCmp{textarea: textarea.New()}
	editor.textarea.Focus()
	editor.Update(tea.PasteMsg{Content: "first line\nsecond line"})
	require.Equal(t, "first line\nsecond line", editor.textarea.Value())
	require.Equal(t, 4, editor.PreferredHeight(60))
}

func TestGrowingEditorRevealsPastedLines(t *testing.T) {
	editor := &editorCmp{textarea: textarea.New(), home: true, app: &app.App{CoderAgent: unconfiguredAgent{}}}
	editor.textarea.Focus()
	editor.SetSize(60, 3)
	editor.View()
	editor.Update(tea.PasteMsg{Content: "first line\nsecond line"})
	editor.SetSize(60, editor.PreferredHeight(60))
	view := ansi.Strip(editor.View())
	require.Contains(t, view, "first line")
	require.Contains(t, view, "second line")
	require.Equal(t, 1, editor.textarea.Line())
	require.Equal(t, len("second line"), editor.textarea.LineInfo().ColumnOffset)
}

func TestTabRequestsProfileSwitchAndPreservesDraft(t *testing.T) {
	editor := &editorCmp{textarea: textarea.New(), attachments: []message.Attachment{{FileName: "draft.txt"}}}
	editor.textarea.Focus()
	editor.textarea.SetValue("keep this draft")
	_, cmd := editor.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	require.NotNil(t, cmd)
	require.IsType(t, ToggleProfileMsg{}, cmd())
	require.Equal(t, "keep this draft", editor.textarea.Value())
	require.Len(t, editor.attachments, 1)
}

func TestTabCompletesCommandBeforeProfileSwitch(t *testing.T) {
	editor := &editorCmp{textarea: textarea.New()}
	editor.textarea.Focus()
	editor.textarea.SetValue("/profil")
	_, cmd := editor.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, "/profiles", editor.textarea.Value())
	require.NotNil(t, cmd)
	require.NotEqual(t, ToggleProfileMsg{}, cmd())
}
