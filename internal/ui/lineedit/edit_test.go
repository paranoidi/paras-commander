package lineedit

import "testing"

func TestEditHelpers(t *testing.T) {
	tests := []struct {
		name    string
		op      func([]rune, int) ([]rune, int)
		in      string
		pos     int
		want    string
		wantPos int
	}{
		{"insert start", func(r []rune, p int) ([]rune, int) { return InsertRune(r, p, 'x') }, "abc", 0, "xabc", 1},
		{"insert mid", func(r []rune, p int) ([]rune, int) { return InsertRune(r, p, 'x') }, "abc", 1, "axbc", 2},
		{"insert end", func(r []rune, p int) ([]rune, int) { return InsertRune(r, p, 'x') }, "abc", 3, "abcx", 4},
		{"backspace at 0", DeleteBefore, "abc", 0, "abc", 0},
		{"backspace at end", DeleteBefore, "abc", 3, "ab", 2},
		{"delete at end", DeleteAt, "abc", 3, "abc", 3},
		{"delete mid", DeleteAt, "abc", 1, "ac", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, pos := tc.op([]rune(tc.in), tc.pos)
			if string(got) != tc.want || pos != tc.wantPos {
				t.Fatalf("got %q,%d want %q,%d", string(got), pos, tc.want, tc.wantPos)
			}
		})
	}
}
