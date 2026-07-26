package textutil

import (
	"errors"
	"testing"

	"github.com/paranoidi/paras-commander/internal/primitive"
)

func TestFirstLine(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"single", "single"},
		{"first\nsecond", "first"},
		{"\n\nmiddle\n", "middle"},
		{"  spaced  ", "spaced"},
	}
	for _, tt := range tests {
		if got := FirstLine(tt.in); got != tt.want {
			t.Errorf("FirstLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	joined := errors.Join(errors.New("alpha"), errors.New("beta"))
	if got := FirstLine(joined.Error()); got != "alpha" {
		t.Fatalf("first line of joined error = %q, want alpha", got)
	}
}

func TestTruncateBannerRunes(t *testing.T) {
	if got := TruncateBannerRunes("short", 10); got != "short" {
		t.Fatalf("got %q, want unchanged", got)
	}
	long := "abcdefghijklmnopqrstuvwxyz"
	got := TruncateBannerRunes(long, 10)
	want := "abcdefghij" + string(primitive.Ellipsis)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
