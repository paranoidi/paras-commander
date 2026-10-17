package ui

import (
	"reflect"
	"testing"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/search"
)

func TestCommandsFilterSegmentsAndRangeSplit(t *testing.T) {
	e := CommandRunEntry{
		UserCommandLine: "bash -c 'pearl %f'",
		Macros:          []cmdmacro.MacroUse{{Letter: 'f', Values: []string{"/home/walnut/meadow/ribbon.txt", "/x"}}},
	}
	segs := CommandsFilterSegments(e, "/home/walnut")
	if want := []string{"'pearl %f'", "~/meadow/ribbon.txt"}; !reflect.DeepEqual(segs, want) {
		t.Fatalf("segments = %q, want %q", segs, want)
	}
	if got, want := CommandsFilterLabel(e, "/home/walnut"), "'pearl %f' ~/meadow/ribbon.txt"; got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}
	// Range 8..15 spans the end of segment 0 (10 runes) and the joining space into segment 1.
	got := splitSegmentRanges(segs, []search.Range{{Start: 8, End: 14}})
	want := [][]search.Range{{{Start: 8, End: 10}}, {{Start: 0, End: 3}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split = %v, want %v", got, want)
	}
}
