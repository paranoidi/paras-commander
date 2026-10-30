package panellist

import (
	"testing"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

var shortenTestNames = []string{
	"Copperfield.Was.Wandering.2041.Orchard.Silver.v7.Lantern",
	"Garden.Lantern.Whisper.2019.Pebble.Draft.odt",
	"quarterly_budget_report_final_revised_v3.xlsx",
	"photo-album-summer-vacation-beach-sunset-0042.jpeg",
	"Meadow Harbor Journal - Chapter 02 Section 07 - The Silver Compass.txt",
	"港口灯笼的故事 第二章 草稿笔记.txt",
	"singlewordwithoutanyseparators",
}

func shortenedText(out []DisplayRune) string { return string(RunesFromDisplay(out)) }

func TestShortenNameExamples(t *testing.T) {
	name := []rune("Copperfield.Was.Wandering.2041.Orchard.Silver.v7.Lantern")
	for _, tc := range []struct {
		width int
		want  string
		cuts  []int // display indexes marked Cut
	}{
		{40, "Coppe.Was.Wande.2041…Silver.v7.Lantern", []int{4, 14, 20}},
		{30, "Coppe.Was.Wande…v7.Lantern", []int{4, 14, 15}},
		{20, "Coppe.Was…Lantern", []int{4, 9}},
		{12, "Coppe…Lant", []int{4, 5, 9}},
	} {
		out := shortenName(name, tc.width, true)
		if got := shortenedText(out); got != tc.want {
			t.Fatalf("width %d: got %q, want %q", tc.width, got, tc.want)
		}
		var cuts []int
		for i, dr := range out {
			if dr.Cut {
				cuts = append(cuts, i)
			}
		}
		if len(cuts) != len(tc.cuts) {
			t.Fatalf("width %d: cuts %v, want %v", tc.width, cuts, tc.cuts)
		}
		for i := range cuts {
			if cuts[i] != tc.cuts[i] {
				t.Fatalf("width %d: cuts %v, want %v", tc.width, cuts, tc.cuts)
			}
		}
	}
}

func TestShortenNameFitsEveryWidth(t *testing.T) {
	for _, n := range shortenTestNames {
		name := []rune(n)
		for w := 1; w <= len(name)*2; w++ {
			if got := displayCells(shortenName(name, w, true)); got > w {
				t.Fatalf("%q width %d: %d cells", n, w, got)
			}
		}
	}
}

// A narrower width shows a subset of what a wider one shows, so names do not reshuffle while a
// panel is resized. Below ~10 cells the extension is given up, which restarts the plan once.
func TestShortenNameNarrowerIsSubset(t *testing.T) {
	for _, n := range shortenTestNames {
		name := []rune(n)
		wider := map[int]bool{}
		for i := range name {
			wider[i] = true
		}
		for w := displayCells(shortenName(name, 1000, true)); w >= 10; w-- {
			shown := map[int]bool{}
			for _, dr := range shortenName(name, w, true) {
				if dr.NameIdx < 0 {
					continue
				}
				if !wider[dr.NameIdx] {
					t.Fatalf("%q width %d shows rune %d hidden at width %d", n, w, dr.NameIdx, w+1)
				}
				shown[dr.NameIdx] = true
			}
			wider = shown
		}
	}
}

func TestShortenNameKeepsNumbersAndExtensionWhole(t *testing.T) {
	name := []rune("photo-album-summer-vacation-beach-sunset-0042.jpeg")
	for w := 12; w < len(name); w++ {
		out := shortenName(name, w, true)
		shown := map[int]bool{}
		for _, dr := range out {
			if dr.NameIdx >= 0 {
				shown[dr.NameIdx] = true
			}
		}
		// "0042" is runes 41..44, ".jpeg" 45..49.
		some := shown[41] || shown[42] || shown[43] || shown[44]
		all := shown[41] && shown[42] && shown[43] && shown[44]
		if some != all {
			t.Fatalf("width %d cuts the number: %q", w, shortenedText(out))
		}
		for i := 45; i < len(name); i++ {
			if !shown[i] {
				t.Fatalf("width %d drops the extension: %q", w, shortenedText(out))
			}
		}
	}
}

func TestShortenNameDirectoryHasNoExtension(t *testing.T) {
	got := shortenedText(shortenName([]rune("lantern-harbor-whisper.archive"), 20, false))
	if got != "lantern…archive" {
		t.Fatalf("got %q", got)
	}
}

func TestEntryDisplayRunesShortensSymlinkKeepingMarkers(t *testing.T) {
	entry := localfs.Entry{Name: "lantern-harbor-whisper-compass.txt", Type: localfs.EntrySymlink}
	got := string(RunesFromDisplay(EntryDisplayRunes(entry, 20, true, RowSuffix{}, theme.Theme{})))
	if want := " lantern…comp.txt@"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestListingCutSpansUseCutMarkStyle(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcell.ColorNavy).Background(tcell.ColorBlack)
	display := shortenName([]rune("Copperfield.Was.Wandering.2041.Orchard.Silver.v7.Lantern"), 20, true)
	spans := ListingCutSpans(display, func(int) tcell.Style { return base })
	if len(spans) != 2 {
		t.Fatalf("spans = %v", spans)
	}
	for _, sp := range spans {
		if fg, bg, _ := sp.Style.Decompose(); fg != tcell.ColorBlue || bg != tcell.ColorBlack {
			t.Fatalf("span style fg=%v bg=%v", fg, bg)
		}
		if r := display[sp.Start].Rune; !unicode.IsLetter(r) && r != '…' {
			t.Fatalf("span on %q", r)
		}
	}
}
