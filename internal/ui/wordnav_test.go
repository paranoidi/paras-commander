package ui

import (
	"reflect"
	"testing"
)

func TestBackwardWordIndex(t *testing.T) {
	r := []rune("/foo/bar")
	tests := []struct {
		pos  int
		want int
	}{
		{len(r), 5}, // end -> before last word
		{5, 1},      // before 'b' -> before 'f'
		{1, 0},      // before 'f' -> BOL
		{0, 0},
		{4, 1}, // before '/' between foo and bar -> before 'f'
	}
	for _, tc := range tests {
		if got := BackwardWordIndex(r, tc.pos); got != tc.want {
			t.Errorf("BackwardWordIndex(%q, %d) = %d, want %d", string(r), tc.pos, got, tc.want)
		}
	}
}

func TestForwardWordIndex(t *testing.T) {
	r := []rune("/foo/bar")
	tests := []struct {
		pos  int
		want int
	}{
		{0, 4},      // skip / then foo
		{1, 4},      // at f
		{4, 8},      // at start of bar segment
		{8, len(r)}, // end of bar
		{len(r), len(r)},
	}
	for _, tc := range tests {
		if got := ForwardWordIndex(r, tc.pos); got != tc.want {
			t.Errorf("ForwardWordIndex(%q, %d) = %d, want %d", string(r), tc.pos, got, tc.want)
		}
	}
}

func TestKillWordBackward(t *testing.T) {
	r := []rune("/foo/bar")
	newR, c := KillWordBackward(r, len(r))
	want := []rune("/foo/")
	if !reflect.DeepEqual(newR, want) || c != 5 {
		t.Fatalf("KillWordBackward(%q, EOT) = %q, cursor %d; want %q cursor 5", string(r), string(newR), c, string(want))
	}
	newR2, c2 := KillWordBackward(newR, len(newR))
	want2 := []rune("/")
	if !reflect.DeepEqual(newR2, want2) || c2 != 1 {
		t.Fatalf("second kill: got %q cursor %d; want %q cursor 1", string(newR2), c2, string(want2))
	}
}

func TestKillWordBackwardNoOpAtBOL(t *testing.T) {
	r := []rune("ab")
	newR, c := KillWordBackward(r, 0)
	if string(newR) != "ab" || c != 0 {
		t.Fatalf("want no-op at BOL, got %q %d", string(newR), c)
	}
}
