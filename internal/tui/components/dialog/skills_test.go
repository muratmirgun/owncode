package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/muratmirgun/owncode/internal/skills"
)

func TestSkillsInputAndBounds(t *testing.T) {
	s := NewSkillsCmp(t.TempDir()).(*skillsCmp)
	s.input.Focus()
	s.entries = []skills.Entry{{Name: "review", Description: "Review Go"}, {Name: "tests", Description: "Test code"}}
	s.Update(tea.PasteMsg{Content: "review"})
	if len(s.filtered()) != 1 {
		t.Fatal("skill filter failed")
	}
	for _, width := range []int{24, 80, 120} {
		s.Update(tea.WindowSizeMsg{Width: width, Height: 25})
		for _, line := range strings.Split(s.View(), "\n") {
			if lipgloss.Width(line) > width {
				t.Fatalf("row exceeds %d columns", width)
			}
		}
	}
	_, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := cmd().(CloseSkillsMsg); !ok {
		t.Fatal("escape did not close")
	}
}
func TestSkillsIgnoreStaleResults(t *testing.T) {
	s := NewSkillsCmp(t.TempDir()).(*skillsCmp)
	s.id = 3
	s.busy = true
	s.Update(skillsResultMsg{id: 2, entries: []skills.Entry{{Name: "stale"}}})
	if len(s.entries) != 0 || !s.busy {
		t.Fatal("stale result changed dialog")
	}
}
