package theme

import (
	"sync/atomic"

	"charm.land/lipgloss/v2"
)

var lightBackground atomic.Bool

// AdaptiveColor preserves theme palettes while implementing image/color.Color.
type AdaptiveColor struct{ Light, Dark string }

func (c AdaptiveColor) RGBA() (uint32, uint32, uint32, uint32) {
	value := c.Dark
	if !IsDark() {
		value = c.Light
	}
	return lipgloss.Color(value).RGBA()
}

// IsDark reports the terminal appearance detected by Bubble Tea.
func IsDark() bool { return !lightBackground.Load() }

// SetDark updates the palette selection after terminal background detection.
func SetDark(dark bool) { lightBackground.Store(!dark) }
