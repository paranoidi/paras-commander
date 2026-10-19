package zoxide

import (
	"slices"
	"testing"
)

func TestParseList(t *testing.T) {
	got := parseList([]byte("/\n/srv/lantern/orchard/\nnot-absolute\n\n/srv/meadow\r\n"))
	want := []string{"/", "/srv/lantern/orchard", "/srv/meadow"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
