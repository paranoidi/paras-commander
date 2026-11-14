package textutil

import (
	"reflect"
	"testing"
)

func TestWrapTokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		width  int
		want   []string
	}{
		{"fits", []string{"rsync", "-a", "src/"}, 20, []string{"rsync -a src/"}},
		{"breaks between tokens", []string{"rsync", "-a", "'--exclude=a b'", "dst/"}, 16, []string{"rsync -a", "'--exclude=a b'", "dst/"}},
		{"exact width stays", []string{"abcd", "efgh"}, 9, []string{"abcd efgh"}},
		{"hard break", []string{"rsync", "abcdefghijkl", "x"}, 5, []string{"rsync", "abcde", "fghij", "kl x"}},
		{"wide runes", []string{"日本語", "日本語"}, 6, []string{"日本語", "日本語"}},
		{"no width", []string{"a", "b"}, 0, []string{"a b"}},
		{"empty", nil, 10, []string{""}},
	}
	for _, tc := range tests {
		if got := WrapTokens(tc.tokens, tc.width); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestClampLines(t *testing.T) {
	lines := []string{"alpha", "bravo", "charlie"}
	if got := ClampLines(lines, 5, 10); !reflect.DeepEqual(got, lines) {
		t.Errorf("room to spare: %q", got)
	}
	if got, want := ClampLines(lines, 2, 10), []string{"alpha", "bravo…"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := ClampLines(lines, 2, 5), []string{"alpha", "brav…"}; !reflect.DeepEqual(got, want) {
		t.Errorf("full last row: got %q, want %q", got, want)
	}
	if got, want := ClampLines(lines, 0, 10), []string{"alpha…"}; !reflect.DeepEqual(got, want) {
		t.Errorf("min one row: got %q, want %q", got, want)
	}
}
