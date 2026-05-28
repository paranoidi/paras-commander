package fsbackend

import "testing"

func TestHasDotfileNames(t *testing.T) {
	t.Parallel()
	if HasDotfileNames(nil) {
		t.Fatal("nil entries: want false")
	}
	if HasDotfileNames([]Entry{{Name: "visible.txt"}}) {
		t.Fatal("no dotfiles: want false")
	}
	if !HasDotfileNames([]Entry{{Name: ".hidden"}}) {
		t.Fatal("dotfile present: want true")
	}
}
