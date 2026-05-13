package dialog

import (
	"strings"
	"testing"
)

func TestWrapMessageBodyBreaksLongWords(t *testing.T) {
	got := wrapMessageBody("abcdefghijklmnop", 6)
	want := []string{"abcdef", "ghijkl", "mnop"}
	if len(got) != len(want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWrapMessageBodyPreservesParagraphBreaks(t *testing.T) {
	got := wrapMessageBody("line one\n\nline two", 20)
	if len(got) < 3 || got[0] != "line one" || got[1] != "" || got[2] != "line two" {
		t.Fatalf("lines = %#v", got)
	}
}

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
