package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestRemapViMotionKey(t *testing.T) {
	cases := []struct {
		rune rune
		want tcell.Key
	}{
		{'h', tcell.KeyLeft},
		{'j', tcell.KeyDown},
		{'k', tcell.KeyUp},
		{'l', tcell.KeyRight},
	}
	for _, tc := range cases {
		in := tcell.NewEventKey(tcell.KeyRune, tc.rune, tcell.ModNone)
		out := RemapViMotionKey(in)
		if out.Key() != tc.want {
			t.Fatalf("remap %q = %v, want %v", tc.rune, out.Key(), tc.want)
		}
	}
}

func TestRemapViMotionKeyPassesThroughOtherRunes(t *testing.T) {
	in := tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone)
	if out := RemapViMotionKey(in); out != in {
		t.Fatalf("non-hjkl rune should pass through unchanged, got %v", out)
	}
}

func TestRemapViMotionKeyIgnoresModifiedHJKL(t *testing.T) {
	// A modified h/j/k/l (e.g. Alt+h) is not a plain vi-motion press; it must pass through
	// unchanged so any chord bound to it keeps working.
	in := tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt)
	if out := RemapViMotionKey(in); out != in {
		t.Fatalf("Alt+h should pass through unchanged, got %v", out)
	}
}

func TestRemapViMotionKeyPassesThroughNonRuneKeys(t *testing.T) {
	in := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	if out := RemapViMotionKey(in); out != in {
		t.Fatalf("non-rune key should pass through unchanged, got %v", out)
	}
}
