package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/tui/util"
	"github.com/stretchr/testify/require"
)

func TestScrollBurstDoesNotBlockKeyboard(t *testing.T) {
	now := time.Unix(100, 0)
	filter := scrollFilter(func() time.Time { return now })
	wheel := tea.MouseWheelMsg{Button: tea.MouseWheelUp}
	require.NotNil(t, filter(nil, wheel))
	for range 1000 {
		require.Nil(t, filter(nil, wheel))
	}
	key := tea.KeyPressMsg{Code: 'a', Text: "a"}
	require.Equal(t, key, filter(nil, key))
	paste := tea.PasteMsg{Content: "draft"}
	require.Equal(t, paste, filter(nil, paste))
	require.NotNil(t, filter(nil, tea.MouseWheelMsg{Button: tea.MouseWheelDown}))
	now = now.Add(40 * time.Millisecond)
	require.NotNil(t, filter(nil, wheel))
}

func TestWheelSamplesAccumulateAndReverse(t *testing.T) {
	now := time.Unix(100, 0)
	filter := scrollFilter(func() time.Time { return now })
	up := tea.MouseWheelMsg{Button: tea.MouseWheelUp}
	require.Equal(t, up, filter(nil, up))
	for range 7 {
		require.Nil(t, filter(nil, up))
	}
	now = now.Add(20 * time.Millisecond)
	scroll := filter(nil, up).(util.ScrollMsg)
	require.Equal(t, 8, scroll.Steps)
	for range 3 {
		require.Nil(t, filter(nil, up))
	}
	down := tea.MouseWheelMsg{Button: tea.MouseWheelDown}
	require.Equal(t, down, filter(nil, down))
}
