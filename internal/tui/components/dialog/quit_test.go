package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestQuitDialogKeepsButtonsOnOneLine(t *testing.T) {
	for _, selectedNo := range []bool{true, false} {
		dialog := &quitDialogCmp{selectedNo: selectedNo}
		view := ansi.Strip(dialog.View())
		require.Contains(t, view, question)
		var buttonRow string
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "Yes") {
				buttonRow = line
			}
		}
		require.Contains(t, buttonRow, "No")
	}
}
