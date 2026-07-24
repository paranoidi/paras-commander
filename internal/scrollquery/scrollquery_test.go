package scrollquery

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestHandleKeyInsertAndBackspace(t *testing.T) {
	var value string
	var cursor, scroll int
	changed := 0
	edit := NewEdit(&value, &cursor, &scroll, 40, func() { changed++ })

	if !HandleKey(nil, tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone), true, edit) {
		t.Fatal("expected rune key to be consumed")
	}
	if value != "a" || changed != 1 {
		t.Fatalf("after insert: value=%q changed=%d, want %q 1", value, changed, "a")
	}

	if !HandleKey(nil, tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone), true, edit) {
		t.Fatal("expected backspace to be consumed")
	}
	if value != "" || changed != 2 {
		t.Fatalf("after backspace: value=%q changed=%d, want %q 2", value, changed, "")
	}
}

func TestHandleKeyIgnoredWhenNotFocused(t *testing.T) {
	var value string
	var cursor, scroll int
	edit := NewEdit(&value, &cursor, &scroll, 40, nil)

	if HandleKey(nil, tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone), false, edit) {
		t.Fatal("expected key to be ignored when input not focused")
	}
	if value != "" {
		t.Fatalf("value mutated while not focused: %q", value)
	}
}

func TestDialogInputWidthFromFrame(t *testing.T) {
	cases := []struct {
		frameWidth int
		want       int
	}{
		{0, 0},
		{3, 0},
		{4, 0},
		{10, 6},
	}
	for _, c := range cases {
		if got := DialogInputWidthFromFrame(c.frameWidth); got != c.want {
			t.Errorf("DialogInputWidthFromFrame(%d) = %d, want %d", c.frameWidth, got, c.want)
		}
	}
}
