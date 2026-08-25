package compare

import "testing"

func TestRowLegendPendingAndHashing(t *testing.T) {
	glyph := "*"
	cases := []struct {
		name string
		row  Row
		want string
	}{
		{
			name: "same-path queued",
			row:  Row{Kind: KindEqual, PrimaryRel: "a.txt", SecondaryRel: "a.txt"},
			want: "* Pending",
		},
		{
			name: "same-path hashing",
			row:  Row{Kind: KindEqual, PrimaryRel: "a.txt", SecondaryRel: "a.txt", Hashing: true},
			want: "* Hashing",
		},
		{
			name: "primary-only queued",
			row:  Row{Kind: KindPrimaryOnly, PrimaryRel: "solo.txt"},
			want: "* Pending",
		},
		{
			name: "secondary-only hashing",
			row:  Row{Kind: KindSecondaryOnly, SecondaryRel: "solo.txt", Hashing: true},
			want: "* Hashing",
		},
		{
			name: "equal done",
			row:  Row{Kind: KindEqual, PrimaryRel: "a.txt", SecondaryRel: "a.txt", HashDone: true},
			want: "* Identical",
		},
		{
			name: "primary-only done",
			row:  Row{Kind: KindPrimaryOnly, PrimaryRel: "solo.txt", HashDone: true},
			want: "* Only on primary",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RowLegend(tc.row, glyph); got != tc.want {
				t.Fatalf("RowLegend = %q, want %q", got, tc.want)
			}
		})
	}
}
