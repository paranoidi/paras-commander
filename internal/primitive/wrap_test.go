package primitive

import "testing"

func TestWrapTextBreaksLongWords(t *testing.T) {
	got := WrapText("abcdefghijklmnop", 6)
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

func TestWrapTextPreservesParagraphBreaks(t *testing.T) {
	got := WrapText("line one\n\nline two", 20)
	if len(got) < 3 || got[0] != "line one" || got[1] != "" || got[2] != "line two" {
		t.Fatalf("lines = %#v", got)
	}
}
