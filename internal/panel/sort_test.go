package panel

import "testing"

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

// The default (Full/Long) format does have a Modified column, so SortMtime
// should still place the arrow there.
func TestListColumnTitles_MtimeArrowOnModifiedInDefaultFormat(t *testing.T) {
	s := State{
		Sort:       SortState{Mode: SortMtime},
		ListFormat: ListFormatMtime,
	}

	_, size, third := s.ListColumnTitles(false)
	if size != "Size" || third != "↑Modified" {
		t.Fatalf("Mtime format/SortMtime: want plain Size and arrowed Modified, got size=%q third=%q", size, third)
	}
}
