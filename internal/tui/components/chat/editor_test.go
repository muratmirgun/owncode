package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
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
		{"", 40, 2},
		{"hello", 40, 2},
		{"one\ntwo\nthree", 40, 4},
		{strings.Repeat("x", 50), 23, 4},
		{strings.Repeat("line\n", 20), 40, 9},
		{"", 40, 2},
	} {
		editor.textarea.SetValue(test.text)
		require.Equal(t, test.height, editor.PreferredHeight(test.width))
	}
	editor.home = true
	require.Equal(t, 3, editor.PreferredHeight(40))
}
