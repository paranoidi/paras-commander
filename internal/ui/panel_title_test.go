package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/primitive"
)

func TestPanelTitlePathHomeTilde(t *testing.T) {
	home := "/home/testuser"
	tests := []struct {
		path string
		want string
	}{
		{"/home/testuser", "~/"},
		{"/home/testuser/", "~/"},
		{"/home/testuser/volumes/foo", "~/volumes/foo"},
	}
	for _, tc := range tests {
		got := PanelTitlePath(tc.path, home, 200)
		if got != tc.want {
			t.Fatalf("PanelTitlePath(%q, home=%q, wide) = %q, want %q", tc.path, home, got, tc.want)
		}
	}
}

func TestPanelTitlePathProgressiveAbbrev(t *testing.T) {
	home := "/home/nobody"
	path := "/home/alice/work/synthetic-repo"
	// No tilde — different user
	got := PanelTitlePath(path, home, len("/home/alice/work/synthetic-repo")-1)
	want := "/h/alice/work/synthetic-repo"
	if got != want {
		t.Fatalf("first abbrev: got %q, want %q", got, want)
	}
	got2 := PanelTitlePath(path, home, len(want)-1)
	want2 := "/h/a/work/synthetic-repo"
	if got2 != want2 {
		t.Fatalf("second abbrev: got %q, want %q", got2, want2)
	}
}

func TestPanelTitlePathTildeThenAbbrev(t *testing.T) {
	home := "/home/testuser"
	path := "/home/testuser/volumes/synthetic-repo"
	max := len("~/volumes/synthetic-repo") - 1
	got := PanelTitlePath(path, home, max)
	want := "~/v/synthetic-repo"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPanelTitlePathEmptyHomeNoTilde(t *testing.T) {
	path := "/home/testuser/foo"
	got := PanelTitlePath(path, "", 200)
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}

func TestPanelTitlePathAbbreviatesURIPathOnly(t *testing.T) {
	t.Cleanup(func() { primitive.SetPathDisplayPrefixes(nil) })
	primitive.SetPathDisplayPrefixes(map[string]string{"fileindex://": " "})
	got := PanelTitlePath("fileindex:///home/wren/orchard/lantern/meadow", "/home/wren", 18)
	if got != " ~/o/l/meadow" {
		t.Fatalf("title = %q, want the scheme kept whole and the path abbreviated", got)
	}
	if got := PanelTitlePath("sftp://user@harbor/srv/orchard/lantern", "", 30); got != "sftp://user@harbor/s/o/lantern" {
		t.Fatalf("sftp title = %q", got)
	}
}

func TestPanelTitleSegments(t *testing.T) {
	home := "/home/plover"
	type want struct {
		text  string
		depth int
	}
	tests := []struct {
		name     string
		path     string
		maxRunes int
		want     []want
	}{
		{"root", "/", 40, []want{{"/", 0}}},
		{"absolute", "/srv/harbor/lantern", 40, []want{{"/", 3}, {"srv", 2}, {"harbor", 1}, {"lantern", 0}}},
		{"home", "/home/plover", 40, []want{{"~", 0}}},
		{"under home", "/home/plover/meadow/copper", 40, []want{{"~", 2}, {"meadow", 1}, {"copper", 0}}},
		{"abbreviated", "/home/plover/meadow/copper", 10, []want{{"~", 2}, {"m", 1}, {"copper", 0}}},
		{"truncated", "/home/plover/meadow/copperwillow", 12, []want{{"~", 2}, {"m", 1}, {"copperw", 0}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const startX = 5
			shown := PanelTitlePath(tc.path, home, tc.maxRunes)
			segs := panelTitleSegments(tc.path, home, startX, tc.maxRunes)
			if len(segs) != len(tc.want) {
				t.Fatalf("segments of %q = %+v, want %d", shown, segs, len(tc.want))
			}
			for i, seg := range segs {
				got := string([]rune(shown)[seg.X-startX : seg.X-startX+seg.Width])
				if got != tc.want[i].text || seg.Depth != tc.want[i].depth {
					t.Fatalf("segment %d of %q = %q depth %d, want %q depth %d", i, shown, got, seg.Depth, tc.want[i].text, tc.want[i].depth)
				}
			}
		})
	}
}

func TestPanelTitleSegmentsSkipDisplayPrefix(t *testing.T) {
	primitive.SetPathDisplayPrefixes(map[string]string{"fileindex://": "# "})
	t.Cleanup(func() { primitive.SetPathDisplayPrefixes(nil) })
	segs := panelTitleSegments("fileindex:///srv/orchid", "", 0, 40)
	want := []PanelTitleSegment{{X: 2, Width: 1, Depth: 2}, {X: 3, Width: 3, Depth: 1}, {X: 7, Width: 6, Depth: 0}}
	if len(segs) != len(want) {
		t.Fatalf("segments = %+v, want %+v", segs, want)
	}
	for i := range want {
		if segs[i] != want[i] {
			t.Fatalf("segments = %+v, want %+v", segs, want)
		}
	}
}
