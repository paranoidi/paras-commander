package panelcarousel

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// fitTestWordEntries builds synthetic listing entries from generic English words of
// deliberately varying length (never real project filenames, per repo test conventions).
// "hippopotamus" (12 runes) is the longest name in the set.
func fitTestWordEntries() []localfs.Entry {
	words := []string{"cat", "otter", "elephant", "hippopotamus", "fox"}
	entries := make([]localfs.Entry, len(words))
	for i, w := range words {
		entries[i] = localfs.Entry{Name: w, Path: "/vol/" + w, Type: localfs.EntryFile}
	}
	return entries
}

func TestCarouselHeaderAlignsWithRowText(t *testing.T) {
	const colWidth = 28
	showIcons := true
	listW := columnListTextWidth(colWidth, showIcons, 0)
	hdr := briefHeader(listNameHeaderTitle(showIcons), "Size", listW, true)
	row := formatBriefRow(localfs.Entry{Name: "another", Type: localfs.EntryDirectory}, colWidth, showIcons, true, panellist.RowSuffix{}, theme.Default(), nil, 0)
	if len([]rune(hdr)) != listW {
		t.Fatalf("header rune width %d, want list text width %d", len([]rune(hdr)), listW)
	}
	if len([]rune(row)) != listW {
		t.Fatalf("row rune width %d, want list text width %d", len([]rune(row)), listW)
	}
	// Name column starts with the same leading space as entry names.
	if !strings.HasPrefix(hdr, " Name") {
		t.Fatalf("header %q should start with leading space before Name", hdr)
	}
	if !strings.HasPrefix(row, " another") {
		t.Fatalf("row %q should start with leading space before entry name", row)
	}
}

func TestBriefHeaderKeepsDiskUsageSortArrow(t *testing.T) {
	const listW = 24
	hdr := briefHeader(" Name", "↓Size", listW, true)
	if !strings.Contains(hdr, "↓Size") {
		t.Fatalf("header %q should contain full ↓Size title", hdr)
	}
}

func TestBriefHeaderNameOnlyWhenSizeHidden(t *testing.T) {
	const listW = 24
	hdr := briefHeader("Name", "", listW, false)
	if strings.Contains(hdr, "Size") {
		t.Fatalf("header %q should not contain size column", hdr)
	}
	if len([]rune(hdr)) != listW {
		t.Fatalf("header width %d, want %d", len([]rune(hdr)), listW)
	}
}

func TestColumnListContentOriginSkipsIconStrip(t *testing.T) {
	const colX = 10
	const colWidth = 20
	x, w := columnListContentOrigin(colX, colWidth, true, 0)
	if x != colX+3 {
		t.Fatalf("list X = %d, want %d (gutter+icon strip)", x, colX+3)
	}
	if w != colWidth-3 {
		t.Fatalf("list width = %d, want %d", w, colWidth-3)
	}
}

func TestColumnListTextWidthReservesScrollbarLane(t *testing.T) {
	const colWidth = 30
	if got := columnListTextWidth(colWidth, false, 1); got != colWidth-1 {
		t.Fatalf("with scrollbar reserve got %d, want %d", got, colWidth-1)
	}
}

func TestColumnScrollbarReserveOnlyWhenScrollNeeded(t *testing.T) {
	t.Parallel()
	if got := columnScrollbarReserve(true, true, uiscrollbar.StyleThumb, 5, 10, 0); got != 0 {
		t.Fatalf("short list reserve = %d, want 0", got)
	}
	if got := columnScrollbarReserve(true, true, uiscrollbar.StyleThumb, 40, 10, 15); got != 1 {
		t.Fatalf("scrollable list reserve = %d, want 1", got)
	}
}

func TestFitEntryTextLen(t *testing.T) {
	t.Parallel()
	if got := fitEntryTextLen(localfs.Entry{Name: "otter", Type: localfs.EntryFile}); got != 1+5 {
		t.Fatalf("fitEntryTextLen(otter) = %d, want %d", got, 1+5)
	}
	if got := fitEntryTextLen(localfs.Entry{Name: "otter", Type: localfs.EntrySymlink}); got != 1+5+1 {
		t.Fatalf("fitEntryTextLen(symlink otter) = %d, want %d", got, 1+5+1)
	}
}

func TestMeasureFitColumnWidthsParentNoIconsNoSize(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFitChars, Value: 16}
	layout.ShowSize[0] = false
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: fitTestWordEntries()}}
	center := panel.State{}

	got := MeasureFitColumnWidths(layout, parent, center, false, true, uiscrollbar.StyleThumb, 10)
	longest := fitEntryTextLen(localfs.Entry{Name: "hippopotamus", Type: localfs.EntryFile})
	want := longest + 1 // +1 right margin
	if got[0] != want {
		t.Fatalf("MeasureFitColumnWidths col[0] = %d, want %d (no icons/size, no scrollbar)", got[0], want)
	}
	if got[1] != 0 || got[2] != 0 {
		t.Fatalf("MeasureFitColumnWidths = %v, want cols 1/2 zero (not fit-mode)", got)
	}
	// Round trip: feeding the measured width back through nameWidthForColumn recovers the
	// longest-name length plus the 1-char right margin (the inverse relationship
	// MeasureFitColumnWidths depends on).
	if nw := nameWidthForColumn(got[0], false, 0, false); nw != want {
		t.Fatalf("nameWidthForColumn round trip = %d, want %d", nw, want)
	}
}

func TestMeasureFitColumnWidthsParentWithIconsAndSize(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFitChars, Value: 64}
	layout.ShowSize[0] = true
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: fitTestWordEntries()}}
	center := panel.State{}

	got := MeasureFitColumnWidths(layout, parent, center, true, true, uiscrollbar.StyleThumb, 10)
	longest := fitEntryTextLen(localfs.Entry{Name: "hippopotamus", Type: localfs.EntryFile})
	want := longest + columnListLeadingGutter() + columnListIconStrip() + 1 + listSizeCells + 1 // +1 right margin
	if got[0] != want {
		t.Fatalf("MeasureFitColumnWidths col[0] = %d, want %d (icons+size, no scrollbar)", got[0], want)
	}
	if nw := nameWidthForColumn(got[0], true, 0, true); nw != longest+1 {
		t.Fatalf("nameWidthForColumn round trip = %d, want %d", nw, longest+1)
	}
}

func TestMeasureFitColumnWidthsReservesScrollbarLane(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFitChars, Value: 64}
	layout.ShowSize[0] = false
	// Nine entries: enough to need a scrollbar at visibleRows=5, but below the p90
	// outlier-cap threshold so fit width stays on the true longest name.
	words := []string{"cat", "otter", "elephant", "hippopotamus", "fox", "gnu", "yak", "seal", "wren"}
	entries := make([]localfs.Entry, len(words))
	for i, w := range words {
		entries[i] = localfs.Entry{Name: w, Path: "/vol/" + w, Type: localfs.EntryFile}
	}
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries}}
	center := panel.State{}
	const visibleRows = 5 // fewer than len(entries): scrollbar lane needed

	got := MeasureFitColumnWidths(layout, parent, center, false, true, uiscrollbar.StyleThumb, visibleRows)
	longest := fitEntryTextLen(localfs.Entry{Name: "hippopotamus", Type: localfs.EntryFile})
	want := longest + 1 /* right margin */ + 1 /* scrollbar reserve */
	if got[0] != want {
		t.Fatalf("MeasureFitColumnWidths col[0] = %d, want %d (+1 margin +1 scrollbar reserve)", got[0], want)
	}
}

func TestMeasureFitColumnWidthsCenterColumn(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[1] = ColumnSplitSpec{Kind: SplitFitPercent, Value: 50}
	layout.ShowSize[1] = false
	center := panel.State{Entries: fitTestWordEntries()}
	parent := Column{}

	got := MeasureFitColumnWidths(layout, parent, center, false, true, uiscrollbar.StyleThumb, 10)
	longest := fitEntryTextLen(localfs.Entry{Name: "hippopotamus", Type: localfs.EntryFile})
	want := longest + 1 // +1 right margin
	if got[1] != want {
		t.Fatalf("MeasureFitColumnWidths col[1] = %d, want %d", got[1], want)
	}
	if got[0] != 0 {
		t.Fatalf("MeasureFitColumnWidths col[0] = %d, want 0 (parent not populated)", got[0])
	}
}

func TestMeasureFitColumnWidthsEmptyColumnFallsBackToZero(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFitChars, Value: 16}
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: nil}}
	center := panel.State{}

	got := MeasureFitColumnWidths(layout, parent, center, false, true, uiscrollbar.StyleThumb, 10)
	if got[0] != 0 {
		t.Fatalf("MeasureFitColumnWidths col[0] = %d, want 0 (empty listing falls back to cap)", got[0])
	}
}

func fitEntriesFromNames(names ...string) []localfs.Entry {
	entries := make([]localfs.Entry, len(names))
	for i, name := range names {
		entries[i] = localfs.Entry{Name: name, Path: "/vol/" + name, Type: localfs.EntryFile}
	}
	return entries
}

func TestFitListingTextLen(t *testing.T) {
	t.Parallel()
	const giant = "extraordinarilylongfilename" // 27 runes → fitEntryTextLen 28
	const giantB = "supercalifragilisticexpial" // 26 runes → fitEntryTextLen 27; near-tied with giant
	// Real-world case: gap 13 but max < 2×second (28 vs 15) — ratio rules miss this.
	const cinnamon = "cinnamon-screensaver-command.sh"
	shorts := []string{
		"cat", "dog", "elk", "fox", "gnu", "hog", "ibis", "jay", "kite", "lion",
		"mole", "newt", "owl", "pig", "quail", "rat", "seal", "toad",
	}
	cases := []struct {
		name  string
		names []string
		want  string // name whose fitEntryTextLen is the expected content length
	}{
		{
			name:  "single outlier ignored",
			names: []string{"cat", "otter", "fox", giant},
			want:  "otter",
		},
		{
			name:  "home-style outlier under 2x second still ignored",
			names: []string{"cat", "otter", "fox", "VirtualBox VMs", cinnamon},
			want:  "VirtualBox VMs",
		},
		{
			name:  "two similar long names keep max when few entries",
			names: []string{"cat", "otter", giant, "supercalifragilistic"},
			want:  giant,
		},
		{
			name:  "two near-tied giants capped by p90",
			names: append(append([]string{}, shorts...), giant, giantB),
			want:  "quail",
		},
		{
			name:  "fewer than three entries keep max",
			names: []string{"otter", giant},
			want:  giant,
		},
		{
			name:  "gradual spread keeps max",
			names: []string{"cat", "otter", "elephant", "hippopotamus"},
			want:  "hippopotamus",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fitListingTextLen(fitEntriesFromNames(tc.names...))
			want := fitEntryTextLen(localfs.Entry{Name: tc.want, Type: localfs.EntryFile})
			if got != want {
				t.Fatalf("fitListingTextLen = %d, want %d (%q)", got, want, tc.want)
			}
		})
	}
}

func TestMeasureFitColumnWidthsIgnoresOutlier(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFitChars, Value: 64, IgnoreOutlier: true}
	layout.ShowSize[0] = false
	const giant = "extraordinarilylongfilename"
	parent := Column{
		Kind:      ColumnParent,
		Populated: true,
		Snapshot:  panel.ListingSnapshot{Entries: fitEntriesFromNames("cat", "otter", "fox", giant)},
	}
	got := MeasureFitColumnWidths(layout, parent, panel.State{}, false, true, uiscrollbar.StyleThumb, 10)
	want := fitEntryTextLen(localfs.Entry{Name: "otter", Type: localfs.EntryFile}) + 1 // +1 right margin
	if got[0] != want {
		t.Fatalf("MeasureFitColumnWidths col[0] = %d, want %d (2nd-longest, not outlier)", got[0], want)
	}
}

func TestMeasureFitColumnWidthsPlainFitKeepsOutlier(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFitChars, Value: 64} // "<64", no IgnoreOutlier
	layout.ShowSize[0] = false
	const giant = "extraordinarilylongfilename"
	parent := Column{
		Kind:      ColumnParent,
		Populated: true,
		Snapshot:  panel.ListingSnapshot{Entries: fitEntriesFromNames("cat", "otter", "fox", giant)},
	}
	got := MeasureFitColumnWidths(layout, parent, panel.State{}, false, true, uiscrollbar.StyleThumb, 10)
	want := fitEntryTextLen(localfs.Entry{Name: giant, Type: localfs.EntryFile}) + 1
	if got[0] != want {
		t.Fatalf("MeasureFitColumnWidths col[0] = %d, want %d (plain fit keeps longest)", got[0], want)
	}
}

func TestCenterNameWidthFitMode(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFlex}
	layout.Splits[1] = ColumnSplitSpec{Kind: SplitFitChars, Value: 40}
	layout.Splits[2] = ColumnSplitSpec{Kind: SplitFlex}
	layout.ShowSize[1] = false
	center := panel.State{Entries: fitTestWordEntries()}
	frame := geom.Rect{X: 0, Y: 0, Width: 120, Height: 20}

	measured := MeasureFitColumnWidths(layout, Column{}, center, false, true, uiscrollbar.StyleThumb, 10)
	longest := fitEntryTextLen(localfs.Entry{Name: "hippopotamus", Type: localfs.EntryFile})
	want := longest + 1 // +1 right margin
	if measured[1] != want {
		t.Fatalf("measured[1] = %d, want %d", measured[1], want)
	}

	got := CenterNameWidth(frame, layout, center, false, true, uiscrollbar.StyleThumb, 10, measured)
	if got != want {
		t.Fatalf("CenterNameWidth = %d, want %d (measured width under the 40-cell cap)", got, want)
	}

	// Zero measured width (unmeasured) falls back to the configured cap.
	gotUnmeasured := CenterNameWidth(frame, layout, center, false, true, uiscrollbar.StyleThumb, 10, [3]int{})
	if gotUnmeasured != 40 {
		t.Fatalf("CenterNameWidth (unmeasured) = %d, want cap 40", gotUnmeasured)
	}
}
