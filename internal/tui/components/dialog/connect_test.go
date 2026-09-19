package dialog

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/muratmirgun/owncode/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestCatalogFailureKeepsLoginForRetryAndClearsOnCancel(t *testing.T) {
	c := NewConnectCmp().(*connectCmp)
	c.id, c.width, c.height = auth.ChatGPT, 80, 24
	connection := auth.Connection{Token: &auth.Token{Access: "private-access"}}
	c.Update(connectResultMsg{owner: c, connection: connection, err: errors.New("empty catalog")})
	require.Equal(t, "retry", c.stage)
	require.Contains(t, c.View(), "login completed")
	require.NotContains(t, c.View(), "private-access")
	_, cmd := c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	require.Equal(t, "waiting", c.stage)
	require.Equal(t, "private-access", c.connection.Token.Access)
	c.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.Nil(t, c.connection.Token)
	require.Equal(t, "providers", c.stage)
}

func TestBrowserLaunchDoesNotInheritTerminalOutput(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		cmd := browserCommand(context.Background(), platform, "https://auth.openai.com/example?state=test")
		require.Nil(t, cmd.Stdout)
		require.Nil(t, cmd.Stderr)
		require.Nil(t, cmd.Stdin)
		require.Equal(t, "https://auth.openai.com/example?state=test", cmd.Args[len(cmd.Args)-1])
	}
}

func TestConnectMasksKeyAndIgnoresCancelledResult(t *testing.T) {
	c := NewConnectCmp().(*connectCmp)
	c.width, c.height, c.selected = 100, 30, 1
	c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	c.Update(tea.PasteMsg{Content: "private-key"})
	require.NotContains(t, c.View(), "private-key")
	require.Contains(t, c.View(), "•")
	oldAttempt := c.attempt
	c.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	c.Update(connectResultMsg{owner: c, attempt: oldAttempt, connection: auth.Connection{Key: "old-key", Models: []auth.Model{{ID: "old"}}}})
	require.Equal(t, "providers", c.stage)
	require.Empty(t, c.key)
	require.Empty(t, c.connection.Key)
}

func TestConnectModelSelectionAndLayout(t *testing.T) {
	c := NewConnectCmp().(*connectCmp)
	c.id = auth.Claude
	connection := auth.Connection{Key: "private", Models: []auth.Model{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}}}
	c.Update(connectResultMsg{owner: c, connection: connection})
	for _, width := range []int{60, 100} {
		c.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		require.LessOrEqual(t, lipgloss.Width(c.View()), width)
		require.LessOrEqual(t, lipgloss.Height(c.View()), 24)
	}
	c.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd := c.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	result := cmd().(ProviderConnectedMsg)
	require.Equal(t, "two", result.Model.ID)
	require.Equal(t, auth.Claude, result.ID)
	require.Empty(t, c.connection.Key)
}
