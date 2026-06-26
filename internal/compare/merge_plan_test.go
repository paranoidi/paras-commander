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
