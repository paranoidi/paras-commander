package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestConfigDialogScrollModeFocus(t *testing.T) {
	t.Parallel()
	want := []int{3, 5, 7}
	for i, focus := range want {
		if got := ConfigDialogScrollModeFocus(i); got != focus {
			t.Fatalf("row %d: focus = %d, want %d", i, got, focus)
		}
	}
}

func TestConfigDialogScrollbarFocus(t *testing.T) {
	t.Parallel()
	want := []int{4, 6, 8}
	for i, focus := range want {
		if got := ConfigDialogScrollbarFocus(i); got != focus {
			t.Fatalf("row %d: focus = %d, want %d", i, got, focus)
		}
	}
}

func TestConfigDialogScrollModeIndex(t *testing.T) {
	t.Parallel()
	for focus, want := range map[int]int{3: 0, 5: 1, 7: 2} {
		idx, ok := ConfigDialogScrollModeIndex(focus)
		if !ok || idx != want {
			t.Fatalf("focus %d: idx=%d ok=%v, want %d true", focus, idx, ok, want)
		}
	}
	for _, focus := range []int{0, 4, 6, 8, 9} {
		if _, ok := ConfigDialogScrollModeIndex(focus); ok {
			t.Fatalf("focus %d: unexpected scroll-mode match", focus)
		}
	}
}

func TestConfigDialogScrollbarIndex(t *testing.T) {
	t.Parallel()
	for focus, want := range map[int]int{4: 0, 6: 1, 8: 2} {
		idx, ok := ConfigDialogScrollbarIndex(focus)
		if !ok || idx != want {
			t.Fatalf("focus %d: idx=%d ok=%v, want %d true", focus, idx, ok, want)
		}
	}
	for _, focus := range []int{0, 3, 5, 7, 9} {
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
		{3, tcell.KeyRight, 4},
		{4, tcell.KeyLeft, 3},
		{5, tcell.KeyRight, 6},
		{7, tcell.KeyRight, 8},
		{3, tcell.KeyDown, 5},
		{5, tcell.KeyDown, 7},
		{7, tcell.KeyDown, 9},
		{4, tcell.KeyDown, 6},
		{8, tcell.KeyDown, 9},
		{8, tcell.KeyUp, 6},
		{7, tcell.KeyUp, 5},
		{3, tcell.KeyUp, 2},
		{4, tcell.KeyUp, 2},
		{8, tcell.KeyLeft, 7},
		{9, tcell.KeyUp, 7},
		{3, tcell.KeyLeft, 3},
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
	if _, ok := ConfigDialogMoveScrollFocus(9, tcell.KeyDown); ok {
		t.Fatal("focus 9 Down should not be handled by scroll mover")
	}
	if _, ok := ConfigDialogMoveScrollFocus(10, tcell.KeyUp); ok {
		t.Fatal("focus 10 Up should fall through to linear form")
	}
	if _, ok := ConfigDialogMoveScrollFocus(3, tcell.KeyTab); ok {
		t.Fatal("Tab should not be handled by scroll mover")
	}
}
