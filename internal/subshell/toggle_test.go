package subshell

import "testing"

func TestSplitOnToggleKey(t *testing.T) {
	before, after, ok := SplitOnToggleKey([]byte("hello\x0fworld"))
	if !ok {
		t.Fatal("expected toggle")
	}
	if string(before) != "hello" || string(after) != "world" {
		t.Fatalf("split = %q | %q", before, after)
	}
}

func TestFindToggleKittyCtrlO(t *testing.T) {
	for _, seq := range [][]byte{
		[]byte("\x1b[111;5u"),
		[]byte("\x1b[111;5:1u"),
		[]byte("\x1b[15;5u"),
		[]byte("\x1b[15;5;1u"),
		[]byte("\x1b[15;5~"),
		[]byte("\x0f"),
	} {
		at, length, ok := FindToggle(seq)
		if !ok || at != 0 || length != len(seq) {
			t.Fatalf("FindToggle(%q) = %d %d %v", seq, at, length, ok)
		}
	}
	for _, seq := range [][]byte{
		[]byte("\x1b[15;5;3u"),
		[]byte("\x1b[111;5;3u"),
	} {
		if _, _, ok := FindToggle(seq); !ok {
			t.Fatalf("release should toggle: %q", seq)
		}
	}
}

func TestSafePTYFlushLenHoldsCtrlOPrefix(t *testing.T) {
	if got := SafePTYFlushLen([]byte("ab\x1b[")); got != 2 {
		t.Fatalf("SafePTYFlushLen = %d, want 2", got)
	}
}

func TestSafePTYFlushLenFlushesArrowKey(t *testing.T) {
	seq := []byte("\x1b[2;1u")
	if got := SafePTYFlushLen(seq); got != len(seq) {
		t.Fatalf("SafePTYFlushLen(%q) = %d, want %d", seq, got, len(seq))
	}
}

func TestSafePTYFlushLenFlushesPlain(t *testing.T) {
	if got := SafePTYFlushLen([]byte("hello")); got != 5 {
		t.Fatalf("SafePTYFlushLen = %d, want 5", got)
	}
}

func TestSafePTYFlushLenFlushesAltC(t *testing.T) {
	if got := SafePTYFlushLen([]byte("\x1bc")); got != 2 {
		t.Fatalf("SafePTYFlushLen = %d, want 2", got)
	}
}
