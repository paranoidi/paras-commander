package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestConfigDialogScrollModeFocus(t *testing.T) {
	t.Parallel()
	want := []int{4, 6, 8}
	for i, focus := range want {
		if got := ConfigDialogScrollModeFocus(i); got != focus {
			t.Fatalf("row %d: focus = %d, want %d", i, got, focus)
		}
	}
}

func TestConfigDialogScrollbarFocus(t *testing.T) {
	t.Parallel()
	want := []int{5, 7, 9}
	for i, focus := range want {
		if got := ConfigDialogScrollbarFocus(i); got != focus {
			t.Fatalf("row %d: focus = %d, want %d", i, got, focus)
		}
	}
}

func TestConfigDialogScrollModeIndex(t *testing.T) {
	t.Parallel()
	for focus, want := range map[int]int{4: 0, 6: 1, 8: 2} {
		idx, ok := ConfigDialogScrollModeIndex(focus)
		if !ok || idx != want {
			t.Fatalf("focus %d: idx=%d ok=%v, want %d true", focus, idx, ok, want)
		}
	}
	for _, focus := range []int{0, 5, 7, 9, 10} {
		if _, ok := ConfigDialogScrollModeIndex(focus); ok {
			t.Fatalf("focus %d: unexpected scroll-mode match", focus)
		}
	}
}

func TestConfigDialogScrollbarIndex(t *testing.T) {
	t.Parallel()
	for focus, want := range map[int]int{5: 0, 7: 1, 9: 2} {
		idx, ok := ConfigDialogScrollbarIndex(focus)
		if !ok || idx != want {
			t.Fatalf("focus %d: idx=%d ok=%v, want %d true", focus, idx, ok, want)
		}
	}
	for _, focus := range []int{0, 4, 6, 8, 10} {
		if _, ok := ConfigDialogScrollbarIndex(focus); ok {
			t.Fatalf("focus %d: unexpected scrollbar match", focus)
		}
	}
}

func TestConfigDialogMoveScrollFocus(t *testing.T) {
	t.Parallel()
	type step struct {
		from int
		key  tcell.Key
		want int
	}
	steps := []step{
		{4, tcell.KeyRight, 5},
		{5, tcell.KeyLeft, 4},
		{6, tcell.KeyRight, 7},
		{8, tcell.KeyRight, 9},
		{4, tcell.KeyDown, 6},
		{6, tcell.KeyDown, 8},
		{8, tcell.KeyDown, 10},
		{5, tcell.KeyDown, 7},
		{9, tcell.KeyDown, 10},
		{9, tcell.KeyUp, 7},
		{8, tcell.KeyUp, 6},
		{4, tcell.KeyUp, 3},
		{5, tcell.KeyUp, 3},
		{9, tcell.KeyLeft, 8},
		{10, tcell.KeyUp, 8},
		{4, tcell.KeyLeft, 4},
	}
	for _, s := range steps {
		got, ok := ConfigDialogMoveScrollFocus(s.from, s.key)
		if !ok {
			t.Fatalf("focus %d key %v: not handled", s.from, s.key)
		}
		if got != s.want {
			t.Fatalf("focus %d key %v: got %d, want %d", s.from, s.key, got, s.want)
		}
	}
	if _, ok := ConfigDialogMoveScrollFocus(10, tcell.KeyDown); ok {
		t.Fatal("focus 10 Down should not be handled by scroll mover")
	}
	if _, ok := ConfigDialogMoveScrollFocus(11, tcell.KeyUp); ok {
		t.Fatal("focus 11 Up should fall through to linear form")
	}
	if _, ok := ConfigDialogMoveScrollFocus(4, tcell.KeyTab); ok {
		t.Fatal("Tab should not be handled by scroll mover")
	}
}
