package panel

import (
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panellist"
)

func TestColumnSetNamesRoundTrip(t *testing.T) {
	for _, set := range []ColumnSet{0, ColumnsBrief, ColumnsModified, ColumnsPermissions, ColumnsBrief.With(ColumnOwner).With(ColumnLinks)} {
		got, unknown := ParseColumnNames(set.Names())
		if got != set || len(unknown) != 0 {
			t.Fatalf("round trip of %v = %v, unknown %v", set.Names(), got.Names(), unknown)
		}
	}
}

func TestParseColumnNamesReturnsUnknown(t *testing.T) {
	set, unknown := ParseColumnNames([]string{"owner", "lantern"})
	if set != ColumnSet(0).With(ColumnOwner) || !slices.Equal(unknown, []string{"lantern"}) {
		t.Fatalf("set = %v, unknown = %v", set.Names(), unknown)
	}
}

func TestColumnSetDropRightmostFollowsDrawnOrder(t *testing.T) {
	set := ColumnsBrief.With(ColumnGroup).With(ColumnModified)
	want := [][]string{{"size", "modified"}, {"size"}, {}}
	for _, names := range want {
		set = set.DropRightmost()
		if !slices.Equal(set.Names(), names) {
			t.Fatalf("names = %v, want %v", set.Names(), names)
		}
	}
	if set.DropRightmost() != 0 {
		t.Fatal("dropping from an empty set must stay empty")
	}
}

func TestColumnSetReservedWidthCountsGaps(t *testing.T) {
	if got, want := ColumnsBrief.ReservedWidth(), panellist.SizeGap+columnSpecs[0].Width; got != want {
		t.Fatalf("brief = %d, want %d", got, want)
	}
	if got, want := ColumnsModified.ReservedWidth(), panellist.SizeGap+columnSpecs[0].Width+columnGap+ColumnTimeCells; got != want {
		t.Fatalf("modified = %d, want %d", got, want)
	}
	if got, want := ColumnSet(0).With(ColumnGroup).ReservedWidth(), columnGap+10; got != want {
		t.Fatalf("group only = %d, want %d", got, want)
	}
}

func TestColumnMnemonicsAreUniqueAndFreeOfButtons(t *testing.T) {
	seen := map[rune]bool{'o': true, 'c': true}
	for _, spec := range Columns() {
		if seen[spec.Mnemonic] {
			t.Fatalf("%s: mnemonic %q taken", spec.Name, spec.Mnemonic)
		}
		seen[spec.Mnemonic] = true
	}
}
