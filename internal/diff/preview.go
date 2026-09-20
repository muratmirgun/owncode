package diff

import (
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/muratmirgun/owncode/internal/tui/theme"
	"strings"
)

// renderPreviewHunk wraps evidence instead of hiding it behind ellipses.
func renderPreviewHunk(h Hunk, width int) string {
	width = max(1, width)
	if width < 90 {
		rows := []string{}
		for _, line := range h.Lines {
			number := line.NewLineNo
			if line.Kind == LineRemoved {
				number = line.OldLineNo
			}
			rows = append(rows, previewCell(&line, number, width)...)
		}
		return strings.Join(rows, "\n") + "\n"
	}
	leftWidth := (width - 1) / 2
	rightWidth := width - 1 - leftWidth
	t := theme.CurrentTheme()
	separator := lipgloss.NewStyle().Background(t.BackgroundSecondary()).Foreground(t.BorderNormal()).Render("│")
	rows := []string{}
	for _, pair := range previewPairs(h.Lines) {
		oldNo, newNo := 0, 0
		if pair.left != nil {
			oldNo = pair.left.OldLineNo
		}
		if pair.right != nil {
			newNo = pair.right.NewLineNo
		}
		left := previewCell(pair.left, oldNo, leftWidth)
		right := previewCell(pair.right, newNo, rightWidth)
		for i := range max(len(left), len(right)) {
			l := previewCell(nil, 0, leftWidth)[0]
			r := previewCell(nil, 0, rightWidth)[0]
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			rows = append(rows, l+separator+r)
		}
	}
	return strings.Join(rows, "\n") + "\n"
}

func previewCell(line *DiffLine, number, width int) []string {
	t := theme.CurrentTheme()
	bg := t.DiffContextBg()
	marker := " "
	if line != nil {
		if line.Kind == LineAdded {
			bg = t.DiffAddedBg()
			marker = "+"
		}
		if line.Kind == LineRemoved {
			bg = t.DiffRemovedBg()
			marker = "-"
		}
	}
	base := lipgloss.NewStyle().Foreground(t.Text()).Background(bg)
	if line == nil {
		return []string{base.Width(width).Render("")}
	}
	gutter := min(8, max(0, width-2))
	prefix := fmt.Sprintf("%5d %s ", number, marker)
	prefix = ansi.Truncate(prefix, gutter, "")
	text := strings.ReplaceAll(ansi.Strip(line.Content), "\t", "    ")
	wrapped := strings.Split(ansi.Wrap(text, max(1, width-gutter), ""), "\n")
	result := make([]string, len(wrapped))
	for i, content := range wrapped {
		label := strings.Repeat(" ", gutter)
		if i == 0 {
			label = prefix
		}
		result[i] = base.Width(width).Render(base.Foreground(t.DiffLineNumber()).Render(label) + base.Render(content))
	}
	return result
}

func previewPairs(lines []DiffLine) []linePair {
	pairs := []linePair{}
	for i := 0; i < len(lines); {
		if lines[i].Kind == LineContext {
			pairs = append(pairs, linePair{left: &lines[i], right: &lines[i]})
			i++
			continue
		}
		removed, added := []*DiffLine{}, []*DiffLine{}
		for i < len(lines) && lines[i].Kind != LineContext {
			if lines[i].Kind == LineRemoved {
				removed = append(removed, &lines[i])
			} else {
				added = append(added, &lines[i])
			}
			i++
		}
		for j := range max(len(removed), len(added)) {
			pair := linePair{}
			if j < len(removed) {
				pair.left = removed[j]
			}
			if j < len(added) {
				pair.right = added[j]
			}
			pairs = append(pairs, pair)
		}
	}
	return pairs
}
