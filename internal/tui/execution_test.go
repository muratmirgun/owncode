package tui

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestModeArguments(t *testing.T) {
	for _, name := range []string{"fast", "yolo"} {
		for _, current := range []bool{false, true} {
			enabled, err := modeValue([]string{name}, current)
			require.NoError(t, err)
			require.Equal(t, !current, enabled)
			for _, arg := range []string{"on", "off"} {
				enabled, err := modeValue([]string{name, arg}, current)
				require.NoError(t, err)
				require.Equal(t, arg == "on", enabled)
			}
			for _, arg := range []string{"yes", "on extra"} {
				enabled, err := modeValue(strings.Fields(name+" "+arg), current)
				require.Error(t, err)
				require.Equal(t, current, enabled)
			}
		}
	}
}
