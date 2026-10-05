package panel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func checkNames(t *testing.T, entries []localfs.Entry, want []string) {
	t.Helper()
	names := entryNames(entries)
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
}

// TestSortEntries_MetaNumeric sorts a meta column whose values all parse as numbers by numeric
// value rather than lexical string order ("10" > "9"): high-first by default, low-first reversed.
func TestSortEntries_MetaNumeric(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "lantern.txt", Path: "/d/lantern.txt"},
		{Name: "meadow.txt", Path: "/d/meadow.txt"},
		{Name: "harbor.txt", Path: "/d/harbor.txt"},
	}
	values := map[string]string{
		"/d/lantern.txt": "9",
		"/d/meadow.txt":  "10",
		"/d/harbor.txt":  "5.6",
	}
	metaValue := func(_ string) (map[string]string, string, bool) {
		return values, "", true
	}
	SortEntries(entries, SortState{Mode: SortMeta, MetaColumn: "size"}, nil, false, metaValue)
	checkNames(t, entries, []string{"meadow.txt", "lantern.txt", "harbor.txt"}) // 10 > 9 > 5.6
	SortEntries(entries, SortState{Mode: SortMeta, MetaColumn: "size", Reverse: true}, nil, false, metaValue)
	checkNames(t, entries, []string{"harbor.txt", "lantern.txt", "meadow.txt"})
}

// TestListColumnTitles_ArrowPointsToLargerValues: ascending shows ↓, descending ↑, and the
// largest-first disk-usage sort ↑.
func TestListColumnTitles_ArrowPointsToLargerValues(t *testing.T) {
	s := State{Sort: SortState{Mode: SortName}}
	if name, _, _ := s.ListColumnTitles(false); name != "↓Name" {
		t.Fatalf("ascending name = %q, want ↓Name", name)
	}
	s.Sort.Reverse = true
	if name, _, _ := s.ListColumnTitles(false); name != "↑Name" {
		t.Fatalf("reversed name = %q, want ↑Name", name)
	}
	s = State{Sort: SortState{DiskUsageIdleSizeSort: true}, IdleDiskTotalsSort: true}
	if _, size, _ := s.ListColumnTitles(false); size != "↑Size" {
		t.Fatalf("disk totals size = %q, want ↑Size", size)
	}
}

// TestSortEntries_MetaMixedNumericText places all-numeric values before non-numeric text,
// regardless of what the text says.
func TestSortEntries_MetaMixedNumericText(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "willow.txt", Path: "/d/willow.txt"},
		{Name: "compass.txt", Path: "/d/compass.txt"},
		{Name: "beacon.txt", Path: "/d/beacon.txt"},
	}
	values := map[string]string{
		"/d/willow.txt":  "apple",
		"/d/compass.txt": "3",
		"/d/beacon.txt":  "banana",
	}
	metaValue := func(_ string) (map[string]string, string, bool) {
		return values, "", true
	}
	SortEntries(entries, SortState{Mode: SortMeta, MetaColumn: "info"}, nil, false, metaValue)
	// numeric ("3") first, then text ordered by value ("apple" < "banana"), not by filename.
	checkNames(t, entries, []string{"compass.txt", "willow.txt", "beacon.txt"})
}

// TestSortEntries_MetaEmptyLast confirms missing/empty meta values sort last regardless of
// Reverse, same rule as the disk-usage idle sort's unknown sizes.
func TestSortEntries_MetaEmptyLast(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "orchid.txt", Path: "/d/orchid.txt"},
		{Name: "gravel.txt", Path: "/d/gravel.txt"},
		{Name: "cinder.txt", Path: "/d/cinder.txt"},
	}
	values := map[string]string{
		"/d/orchid.txt": "2",
		"/d/gravel.txt": "",
		// cinder.txt has no entry at all (metaValue returns ok=false)
	}
	metaValue := func(_ string) (map[string]string, string, bool) {
		return values, "", true
	}

	for _, reverse := range []bool{false, true} {
		e := append([]localfs.Entry(nil), entries...)
		SortEntries(e, SortState{Mode: SortMeta, MetaColumn: "n", Reverse: reverse}, nil, false, metaValue)
		names := entryNames(e)
		if names[len(names)-1] != "gravel.txt" && names[len(names)-1] != "cinder.txt" {
			t.Fatalf("reverse=%v: last entry = %q, want one of the empty/missing entries", reverse, names[len(names)-1])
		}
		if names[0] != "orchid.txt" {
			t.Fatalf("reverse=%v: first entry = %q, want orchid.txt (the only known value)", reverse, names[0])
		}
	}
}

// Regression for bug: cycling listing formats left the sort arrow on an
// unrelated column (Size/Permissions) when SortMtime is active in a format
// that has no Modified column at all (Brief, Perm).
func TestListColumnTitles_MtimeArrowOmittedWithoutModifiedColumn(t *testing.T) {
	s := State{
		Sort:       SortState{Mode: SortMtime},
		ListFormat: ListFormatBrief,
	}

	name, size, third := s.ListColumnTitles(false)
	if size != "Size" || third != "" {
		t.Fatalf("Brief/SortMtime: want arrow-less Size and empty third column, got name=%q size=%q third=%q", name, size, third)
	}

	s.ListFormat = ListFormatPerm
	name, size, third = s.ListColumnTitles(false)
	if size != "Size" || third != "Permissions" {
		t.Fatalf("Perm/SortMtime: want arrow-less Size and Permissions, got name=%q size=%q third=%q", name, size, third)
	}
}

// SortMeta must never put an arrow on the built-in Name/Size/Modified/Permissions columns; the
// arrow for a meta sort lives on the meta column's own header instead (see panelListHeader).
func TestListColumnTitles_MetaArrowOmittedOnBuiltinColumns(t *testing.T) {
	s := State{
		Sort:       SortState{Mode: SortMeta, MetaColumn: "size"},
		ListFormat: ListFormatMtime,
	}
	name, size, third := s.ListColumnTitles(false)
	if name != "Name" || size != "Size" || third != "Modified" {
		t.Fatalf("SortMeta: want arrow-less columns, got name=%q size=%q third=%q", name, size, third)
	}
}

// The default (Full/Long) format does have a Modified column, so SortMtime
// should still place the arrow there.
func TestListColumnTitles_MtimeArrowOnModifiedInDefaultFormat(t *testing.T) {
	s := State{
		Sort:       SortState{Mode: SortMtime},
		ListFormat: ListFormatMtime,
	}

	_, size, third := s.ListColumnTitles(false)
	if size != "Size" || third != "↓Modified" {
		t.Fatalf("Mtime format/SortMtime: want plain Size and arrowed Modified, got size=%q third=%q", size, third)
	}
}

// TestListingFullyDiskCached_OnlyDirsNeedTotals proves non-directories (files created after the
// scan, broken symlinks the walk cannot stat) don't hold off the disk-usage sort, nor do
// directories the scan is known to skip, while an unscanned directory still does; and that the
// disk-usage order falls back to an uncached file's listed size.
func TestListingFullyDiskCached_OnlyDirsNeedTotals(t *testing.T) {
	t.Parallel()
	cached := map[string]int64{"/w/harbor": 900}
	s := &State{
		Entries: []localfs.Entry{
			{Name: "harbor", Path: "/w/harbor", Type: localfs.EntryDirectory},
			{Name: "lantern.bin", Path: "/w/lantern.bin", Type: localfs.EntryFile, Size: 5000},
			{Name: "orphan", Path: "/w/orphan", Type: localfs.EntrySymlink, Size: 12},
			{Name: "meadow", Path: "/w/meadow", Type: localfs.EntryDirectory},
		},
		DiskSorter: func(p string) (int64, bool) { n, ok := cached[p]; return n, ok },
	}
	if s.ListingFullyDiskCached() {
		t.Fatal("unscanned directory must keep the listing unresolved")
	}
	s.DiskExcluded = func(p string) bool { return p == "/w/meadow" }
	if !s.ListingFullyDiskCached() {
		t.Fatal("files, broken symlinks and excluded dirs must not block the disk-usage sort")
	}
	SortEntries(s.Entries, SortState{}, s.DiskSorter, true, nil)
	files := &State{Entries: []localfs.Entry{
		{Name: "lantern.bin", Path: "/w/lantern.bin", Type: localfs.EntryFile, Size: 5000},
		{Name: "orphan", Path: "/w/orphan", Type: localfs.EntrySymlink, Size: 12},
	}, DiskSorter: s.DiskSorter}
	if files.ListingFullyDiskCached() {
		t.Fatal("files-only listing with nothing cached was never scanned and must stay unresolved")
	}
	checkNames(t, s.Entries, []string{"lantern.bin", "harbor", "orphan", "meadow"})
}
