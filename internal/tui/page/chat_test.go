package page

import (
	"testing"

	"github.com/muratmirgun/owncode/internal/app"
	"github.com/muratmirgun/owncode/internal/llm/agent"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/tui/util"
	"github.com/stretchr/testify/require"
)

type unconfiguredAgent struct{ agent.Service }

func (unconfiguredAgent) Model() models.Model { return models.Model{} }

func TestSendWithoutModelDoesNotCreateSession(t *testing.T) {
	t.Parallel()
	page := &chatPage{app: &app.App{CoderAgent: unconfiguredAgent{}}}
	cmd := page.sendMessage("hello", nil)
	require.NotNil(t, cmd)
	_, ok := cmd().(util.InfoMsg)
	require.True(t, ok)
	require.Empty(t, page.session.ID)
}
