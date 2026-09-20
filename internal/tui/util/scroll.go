package util

import tea "charm.land/bubbletea/v2"

// ScrollMsg carries multiple wheel samples through one UI update.
type ScrollMsg struct {
	Wheel tea.MouseWheelMsg
	Steps int
}
