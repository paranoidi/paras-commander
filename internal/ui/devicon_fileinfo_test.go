package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestDeviconHexForeground(t *testing.T) {
	got, ok := deviconHexForeground("#AABBCC")
	if !ok {
		t.Fatal("expected parse ok")
	}
	if want := tcell.NewRGBColor(0xAA, 0xBB, 0xCC); got != want {
		t.Fatalf("color = %v, want %v", got, want)
	}
	if _, ok := deviconHexForeground("invalid"); ok {
		t.Fatal("expected parse fail for invalid hex")
	}
}
