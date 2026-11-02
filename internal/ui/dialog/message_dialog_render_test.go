package dialog

import (
	"strings"
	"testing"
)

func TestTruncateMessageLinesAddsEllipsis(t *testing.T) {
	lines := make([]string, messageDialogMaxBodyLines+4)
	for i := range lines {
		lines[i] = "one-line filler text here"
	}
	out := truncateMessageLines(lines, 30)
	if len(out) != messageDialogMaxBodyLines {
		t.Fatalf("len = %d, want %d", len(out), messageDialogMaxBodyLines)
	}
	last := out[len(out)-1]
	if !strings.HasSuffix(last, " ...") {
		t.Fatalf("last line %q should end with ellipsis", last)
	}
}
