package panellist

import (
	"testing"
	"unicode/utf8"
)

func TestJoinRowBranches(t *testing.T) {
	t.Parallel()
	const nameWidth = 10
	name := "marigold"
	meta := "wander"

	cases := []struct {
		name       string
		showMeta   bool
		showSize   bool
		wantExtras int // extra rune width beyond nameWidth
	}{
		{"meta+size", true, true, 2 + len(meta) + 1 + SizeCells},
		{"meta only", true, false, 2 + len(meta)},
		{"size only", false, true, 1 + SizeCells},
		{"name only", false, false, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := JoinRow(nameWidth, name, meta, tc.showMeta, "12K", tc.showSize)
			want := nameWidth + tc.wantExtras
			if n := utf8.RuneCountInString(got); n != want {
				t.Fatalf("JoinRow(%v,%v) rune width = %d, want %d (row %q)", tc.showMeta, tc.showSize, n, want, got)
			}
		})
	}
}

func TestFormatByteSizeCompactExamples(t *testing.T) {
	t.Parallel()
	if got := FormatByteSizeCompact(0, SizeCells); got != "0" {
		t.Fatalf("FormatByteSizeCompact(0) = %q, want %q", got, "0")
	}
	if got := FormatByteSizeCompact(5000, SizeCells); got != "4.9K" {
		t.Fatalf("FormatByteSizeCompact(5000) = %q, want %q", got, "4.9K")
	}
}
