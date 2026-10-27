package compare_test

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestBuildMergePlanCopyMissingTowardSecondary(t *testing.T) {
	rootA := filepath.Join(t.TempDir(), "a")
	rootB := filepath.Join(t.TempDir(), "b")
	snap := compare.Snapshot{
		PrimaryRoot:   pathloc.MustParse(rootA),
		SecondaryRoot: pathloc.MustParse(rootB),
		Rows: []compare.Row{
			{Kind: compare.KindPrimaryOnly, PrimaryRel: "solo.txt", Size: 10, HashDone: true},
		},
	}
	opts := compare.MergeOptions{
		Direction:   compare.MergeTowardSecondary,
		CopyMissing: true,
	}
	plan, err := compare.BuildMergePlan(snap, snap.Rows, compare.MergeInput{}, opts)
	if err != nil {
		t.Fatalf("BuildMergePlan: %v", err)
	}
	if len(plan.Copies) != 1 {
		t.Fatalf("copies = %d, want 1", len(plan.Copies))
	}
	wantDst := filepath.Join(rootB, "solo.txt")
	if plan.Copies[0].Dst != wantDst {
		t.Fatalf("dst = %q, want %q", plan.Copies[0].Dst, wantDst)
	}
}

func TestBuildMergePlanMoveModeMatchesCopyMode(t *testing.T) {
	rootA := filepath.Join(t.TempDir(), "river")
	rootB := filepath.Join(t.TempDir(), "forest")
	snap := compare.Snapshot{
		PrimaryRoot:   pathloc.MustParse(rootA),
		SecondaryRoot: pathloc.MustParse(rootB),
		Rows: []compare.Row{
			{Kind: compare.KindPrimaryOnly, PrimaryRel: "maple.txt", Size: 20, HashDone: true},
		},
	}
	basOpts := compare.MergeOptions{Direction: compare.MergeTowardSecondary, CopyMissing: true}
	moveOpts := compare.MergeOptions{Direction: compare.MergeTowardSecondary, CopyMissing: true, MoveMode: true}

	planCopy, err := compare.BuildMergePlan(snap, snap.Rows, compare.MergeInput{}, basOpts)
	if err != nil {
		t.Fatalf("BuildMergePlan (copy): %v", err)
	}
	planMove, err := compare.BuildMergePlan(snap, snap.Rows, compare.MergeInput{}, moveOpts)
	if err != nil {
		t.Fatalf("BuildMergePlan (move): %v", err)
	}
	if len(planCopy.Copies) != len(planMove.Copies) {
		t.Fatalf("copy mode copies=%d, move mode copies=%d, want equal", len(planCopy.Copies), len(planMove.Copies))
	}
	if planCopy.Copies[0].Src != planMove.Copies[0].Src || planCopy.Copies[0].Dst != planMove.Copies[0].Dst {
		t.Fatalf("copy item mismatch: copy=%+v move=%+v", planCopy.Copies[0], planMove.Copies[0])
	}
}

func relocatedSnap(t *testing.T) (compare.Snapshot, string, string) {
	t.Helper()
	rootA := filepath.Join(t.TempDir(), "orchard")
	rootB := filepath.Join(t.TempDir(), "willow")
	snap := compare.Snapshot{
		PrimaryRoot:   pathloc.MustParse(rootA),
		SecondaryRoot: pathloc.MustParse(rootB),
		Rows: []compare.Row{
			{Kind: compare.KindRelocated, PrimaryRel: "harbor/compass.txt", SecondaryRel: "compass.txt", Size: 7, HashDone: true},
			{Kind: compare.KindRelocated, PrimaryRel: "ribbon.txt", SecondaryRel: "attic/ribbon-old.txt", Size: 9, HashDone: true},
		},
	}
	return snap, rootA, rootB
}

func TestBuildMergePlanRelocatedKeepsDestinationNamesByDefault(t *testing.T) {
	snap, _, _ := relocatedSnap(t)
	opts := compare.MergeOptions{Direction: compare.MergeTowardSecondary, CopyMissing: true, CopyModified: true}
	if _, err := compare.BuildMergePlan(snap, snap.Rows, compare.MergeInput{}, opts); err == nil {
		t.Fatal("BuildMergePlan: want nothing-to-do error when relocated rows keep destination names")
	}
}

func TestBuildMergePlanRelocatedRenamesInsideDestination(t *testing.T) {
	snap, rootA, rootB := relocatedSnap(t)
	tests := []struct {
		name string
		opts compare.MergeOptions
		want []compare.MergeCopyItem
	}{
		{
			name: "source names toward right rename right",
			opts: compare.MergeOptions{Direction: compare.MergeTowardSecondary, RelocatedUseSource: true},
			want: []compare.MergeCopyItem{
				{Src: filepath.Join(rootB, "compass.txt"), Dst: filepath.Join(rootB, "harbor/compass.txt")},
				{Src: filepath.Join(rootB, "attic/ribbon-old.txt"), Dst: filepath.Join(rootB, "ribbon.txt")},
			},
		},
		{
			name: "source names toward left rename left",
			opts: compare.MergeOptions{Direction: compare.MergeTowardPrimary, RelocatedUseSource: true},
			want: []compare.MergeCopyItem{
				{Src: filepath.Join(rootA, "harbor/compass.txt"), Dst: filepath.Join(rootA, "compass.txt")},
				{Src: filepath.Join(rootA, "ribbon.txt"), Dst: filepath.Join(rootA, "attic/ribbon-old.txt")},
			},
		},
		{
			name: "per-row override beats the default",
			opts: compare.MergeOptions{
				Direction:          compare.MergeTowardSecondary,
				RelocatedUseSource: true,
				RelocatedNames: map[compare.RowKey]compare.NameSide{
					snap.Rows[0].Key(): compare.NameSecondary, // keep the right name
				},
			},
			want: []compare.MergeCopyItem{
				{Src: filepath.Join(rootB, "attic/ribbon-old.txt"), Dst: filepath.Join(rootB, "ribbon.txt")},
			},
		},
		{
			name: "per-row override opts in under keep-destination default",
			opts: compare.MergeOptions{
				Direction: compare.MergeTowardSecondary,
				RelocatedNames: map[compare.RowKey]compare.NameSide{
					snap.Rows[1].Key(): compare.NamePrimary,
				},
			},
			want: []compare.MergeCopyItem{
				{Src: filepath.Join(rootB, "attic/ribbon-old.txt"), Dst: filepath.Join(rootB, "ribbon.txt")},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := compare.BuildMergePlan(snap, snap.Rows, compare.MergeInput{}, tc.opts)
			if err != nil {
				t.Fatalf("BuildMergePlan: %v", err)
			}
			if len(plan.Copies) != 0 || plan.TotalBytes != 0 {
				t.Fatalf("copies = %v bytes %d, want none (renames move no data)", plan.Copies, plan.TotalBytes)
			}
			if len(plan.Renames) != len(tc.want) {
				t.Fatalf("renames = %v, want %v", plan.Renames, tc.want)
			}
			for i := range tc.want {
				if plan.Renames[i] != tc.want[i] {
					t.Fatalf("rename[%d] = %+v, want %+v", i, plan.Renames[i], tc.want[i])
				}
			}
		})
	}
}
