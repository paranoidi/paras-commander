package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestJobsProgressStylesInactiveForPausedAndCanceled(t *testing.T) {
	t.Parallel()
	th := Default()

	activeFill := th.JobsProgressFill
	inactiveFill := th.JobsProgressInactiveFill
	if stylesEqual(activeFill, inactiveFill) {
		t.Fatal("active and inactive progress fill styles must differ")
	}

	for _, status := range []string{"paused", "canceled"} {
		fill, track, labelOnFill, labelOnTrack := th.JobsProgressStyles(status)
		if !stylesEqual(fill, th.JobsProgressInactiveFill) {
			t.Fatalf("%q: fill style mismatch", status)
		}
		if !stylesEqual(track, th.JobsProgressInactiveTrack) {
			t.Fatalf("%q: track style mismatch", status)
		}
		if !stylesEqual(labelOnFill, th.JobsProgressInactiveLabelOnFill) {
			t.Fatalf("%q: labelOnFill style mismatch", status)
		}
		if !stylesEqual(labelOnTrack, th.JobsProgressInactiveLabelOnTrack) {
			t.Fatalf("%q: labelOnTrack style mismatch", status)
		}
	}

	fill, _, _, _ := th.JobsProgressStyles("running")
	if !stylesEqual(fill, th.JobsProgressFill) {
		t.Fatal("running job should use active progress fill")
	}
}

func stylesEqual(a, b tcell.Style) bool {
	aFG, aBG, aAttrs := a.Decompose()
	bFG, bBG, bAttrs := b.Decompose()
	return aFG == bFG && aBG == bBG && aAttrs == bAttrs
}
