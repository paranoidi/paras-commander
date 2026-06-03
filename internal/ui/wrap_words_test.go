package ui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrapWordsToWidthUserMenuTomlExample(t *testing.T) {
	msg := `User menu: menu.toml: toml: line 1 (last key "shell_patterns"): incompatible types: TOML value has type int64; destination has type boolean`
	lines := WrapWordsToWidth(msg, MessageLogWrapRunes)
	if len(lines) < 2 {
		t.Fatalf("got %d lines, want at least 2: %q", len(lines), lines)
	}
	for _, ln := range lines {
		if utf8.RuneCountInString(ln) > MessageLogWrapRunes {
			t.Fatalf("line longer than %d: %q", MessageLogWrapRunes, ln)
		}
	}
	if !strings.Contains(lines[0], "User menu:") {
		t.Fatalf("first line = %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "boolean") {
		t.Fatalf("last line = %q", lines[len(lines)-1])
	}
}

func TestWrapWordsToWidthShortUnchanged(t *testing.T) {
	s := "hello world"
	lines := WrapWordsToWidth(s, 80)
	if len(lines) != 1 || lines[0] != s {
		t.Fatalf("got %#v", lines)
	}
}

func TestWrapWordsToWidthLongWordHardBreak(t *testing.T) {
	got := WrapWordsToWidth("abcdefghijklmnop", 6)
	want := []string{"abcdef", "ghijkl", "mnop"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q want %q", i, got[i], want[i])
		}
	}
}

func TestWrapWordsToWidthNarrowWidth(t *testing.T) {
	got := WrapWordsToWidth("aa bb cc dd", 5)
	want := []string{"aa bb", "cc dd"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q want %q", i, got[i], want[i])
		}
	}
}

func TestWrapTextLinesLongLineWraps(t *testing.T) {
	t.Parallel()
	msg := `User menu: menu.toml: toml: line 1 (last key "shell_patterns"): incompatible types: TOML value has type int64; destination has type boolean`
	got := WrapTextLines(msg, 40)
	if len(got) < 2 {
		t.Fatalf("got %d lines, want at least 2: %q", len(got), got)
	}
	for _, ln := range got {
		if utf8.RuneCountInString(ln) > 40 {
			t.Fatalf("line longer than 40: %q", ln)
		}
	}
	if !strings.Contains(got[0], "User menu:") {
		t.Fatalf("first line = %q", got[0])
	}
}

func TestWrapTextLinesPreservesNewlines(t *testing.T) {
	t.Parallel()
	got := WrapTextLines("line one\n\nline two", 20)
	want := []string{"line one", "", "line two"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q want %q", i, got[i], want[i])
		}
	}
}

func TestWrapTextLinesHardBreaksLongWord(t *testing.T) {
	t.Parallel()
	got := WrapTextLines("abcdefghijklmnop", 6)
	want := []string{"abcdef", "ghijkl", "mnop"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q want %q", i, got[i], want[i])
		}
	}
}
