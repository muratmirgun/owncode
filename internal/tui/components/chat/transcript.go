package chat

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/muratmirgun/owncode/internal/tui/util"
)

type transcriptItem struct {
	content string
	lines   []string
	start   int
}

// transcriptViewport retains item lines and passes only visible rows to Bubbles.
// A scroll never reparses the complete transcript.
type transcriptViewport struct {
	viewport.Model
	items         []transcriptItem
	total, offset int
}

func newTranscriptViewport() transcriptViewport {
	return transcriptViewport{Model: viewport.New()}
}

func (v *transcriptViewport) SetItems(messages []uiMessage) {
	old := v.items
	v.items = make([]transcriptItem, len(messages))
	v.total = 0
	for i, msg := range messages {
		item := transcriptItem{content: msg.content, start: v.total}
		if i < len(old) && old[i].content == msg.content {
			item.lines = old[i].lines
		} else {
			item.lines = strings.Split(msg.content, "\n")
		}
		v.items[i] = item
		v.total += len(item.lines) + 1
	}
	v.offset = min(v.offset, v.maxOffset())
}

func (v transcriptViewport) maxOffset() int { return max(0, v.total-v.Height()) }
func (v transcriptViewport) YOffset() int   { return v.offset }
func (v transcriptViewport) AtBottom() bool { return v.offset >= v.maxOffset() }
func (v *transcriptViewport) GotoBottom()   { v.offset = v.maxOffset() }

func (v transcriptViewport) Update(msg tea.Msg) (transcriptViewport, tea.Cmd) {
	delta := 0
	switch msg := msg.(type) {
	case util.ScrollMsg:
		if msg.Wheel.Button == tea.MouseWheelUp {
			delta = -3 * msg.Steps
		}
		if msg.Wheel.Button == tea.MouseWheelDown {
			delta = 3 * msg.Steps
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			delta = -3
		case tea.MouseWheelDown:
			delta = 3
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, v.KeyMap.PageUp):
			delta = -v.Height()
		case key.Matches(msg, v.KeyMap.PageDown):
			delta = v.Height()
		case key.Matches(msg, v.KeyMap.HalfPageUp):
			delta = -max(1, v.Height()/2)
		case key.Matches(msg, v.KeyMap.HalfPageDown):
			delta = max(1, v.Height()/2)
		}
	}
	v.offset = min(v.maxOffset(), max(0, v.offset+delta))
	return v, nil
}

func (v transcriptViewport) View() string {
	rows := make([]string, 0, max(0, v.Height()))
	index := sort.Search(len(v.items), func(i int) bool {
		return v.items[i].start+len(v.items[i].lines)+1 > v.offset
	})
	for ; index < len(v.items) && len(rows) < v.Height(); index++ {
		item := v.items[index]
		for line := max(0, v.offset-item.start); line <= len(item.lines) && len(rows) < v.Height(); line++ {
			if line == len(item.lines) {
				rows = append(rows, "")
			} else {
				rows = append(rows, item.lines[line])
			}
		}
	}
	if v.Width() <= 0 || v.Height() <= 0 {
		return ""
	}
	screen := uv.NewScreenBuffer(v.Width(), v.Height())
	uv.NewStyledString(strings.Join(rows, "\n")).Draw(screen, screen.Bounds())
	return screen.Render()
}

func (v transcriptViewport) GetContent() string {
	items := make([]string, len(v.items))
	for i, item := range v.items {
		items[i] = item.content
	}
	return strings.Join(items, "\n\n") + "\n"
}
