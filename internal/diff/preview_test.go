package diff

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestPreviewWrapsAndAlignsChanges(t *testing.T) {
	text := "--- a/README.md\n+++ b/README.md\n@@ -730,3 +730,3 @@\n context\n-old one\n-old two\n+new one\n+" + strings.Repeat("wide 界 text ", 20) + "END\n"
	parsed, err := ParseUnifiedDiff(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Hunks) != 1 || len(parsed.Hunks[0].Lines) != 5 {
		t.Fatal("parser added phantom lines")
	}
	pairs := previewPairs(parsed.Hunks[0].Lines)
	if len(pairs) != 3 || pairs[1].left.Content != "old one" || pairs[1].right.Content != "new one" {
		t.Fatal("replacement blocks misaligned")
	}
	for _, width := range []int{35, 100, 160} {
		rendered, err := FormatDiff(text, WithTotalWidth(width))
		if err != nil {
			t.Fatal(err)
		}
		plain := ansi.Strip(rendered)
		if !strings.Contains(plain, "END") || !strings.Contains(plain, "730") {
			t.Fatalf("width %d lost content: %q", width, plain)
		}
		for _, line := range strings.Split(strings.TrimSuffix(plain, "\n"), "\n") {
			if lipgloss.Width(line) > width {
				t.Fatalf("row width %d exceeds %d", lipgloss.Width(line), width)
			}
		}
		if (width >= 90) != strings.Contains(plain, "│") {
			t.Fatalf("wrong layout at width %d", width)
		}
	}
}
