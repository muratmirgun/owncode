package chat

import (
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
