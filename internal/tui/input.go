package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

// ScrollFilter limits redundant trackpad events before they trigger a full view.
// Keyboard input always passes through immediately.
func ScrollFilter() func(tea.Model, tea.Msg) tea.Msg {
	return scrollFilter(time.Now)
}

func scrollFilter(now func() time.Time) func(tea.Model, tea.Msg) tea.Msg {
	var last time.Time
	var lastSample time.Time
	var previous tea.MouseWheelMsg
	pending := 0
	return func(_ tea.Model, msg tea.Msg) tea.Msg {
		wheel, ok := msg.(tea.MouseWheelMsg)
		if !ok {
			pending = 0
			return msg
		}
		at := now()
		if at.Sub(lastSample) > 100*time.Millisecond {
			pending = 0
		}
		lastSample = at
		if wheel.Button != previous.Button || wheel.Mod != previous.Mod || wheel.X != previous.X || wheel.Y != previous.Y {
			pending = 0
		}
		pending++
		if !last.IsZero() && wheel.Button == previous.Button && wheel.Mod == previous.Mod && at.Sub(last) < time.Second/60 {
			return nil
		}
		last, previous = at, wheel
		steps := pending
		pending = 0
		if steps > 1 {
			return util.ScrollMsg{Wheel: wheel, Steps: steps}
		}
		return msg
	}
}
