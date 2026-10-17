package compare

import (
	"reflect"
	"testing"
)

func TestPruneRows(t *testing.T) {
	both := Row{Kind: KindContentDiff, PrimaryRel: "harbor/lantern.txt", SecondaryRel: "harbor/lantern.txt", Size: 7, HashDone: true}
	reloc := Row{Kind: KindRelocated, PrimaryRel: "meadow/pebble.txt", SecondaryRel: "orchard/pebble.txt", Size: 3, HashDone: true}
	pOnly := Row{Kind: KindPrimaryOnly, PrimaryRel: "harbor/velvet.txt", Size: 5, HashDone: true}
	sOnly := Row{Kind: KindSecondaryOnly, SecondaryRel: "cedar/maple.txt", Size: 9}
	skipped := Row{Kind: KindSkipped, PrimaryRel: "harbor/thistle.txt", SecondaryRel: "harbor/thistle.txt", Err: "denied"}
	rows := []Row{both, reloc, pOnly, sOnly, skipped}

	tests := []struct {
		name string
		p, s []string
		want []Row
	}{
		{"none", nil, nil, rows},
		{"primary dir drops one-sided and downgrades pairs",
			[]string{"harbor"}, nil,
			[]Row{
				{Kind: KindSecondaryOnly, SecondaryRel: "harbor/lantern.txt", Size: 7, HashDone: true},
				reloc, sOnly,
				{Kind: KindSecondaryOnly, SecondaryRel: "harbor/thistle.txt", HashDone: true},
			}},
		{"secondary file downgrades to primary only",
			nil, []string{"orchard/pebble.txt"},
			[]Row{both, {Kind: KindPrimaryOnly, PrimaryRel: "meadow/pebble.txt", Size: 3, HashDone: true}, pOnly, sOnly, skipped}},
		{"both sides drop row",
			[]string{"harbor/lantern.txt"}, []string{"harbor/lantern.txt", "cedar/maple.txt"},
			[]Row{reloc, pOnly, skipped}},
		{"prefix is path-bounded",
			[]string{"harb"}, []string{"cedar/map"}, rows},
	}
	for _, tc := range tests {
		got := PruneRows(append([]Row(nil), rows...), tc.p, tc.s)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.name, got, tc.want)
		}
	}
}
