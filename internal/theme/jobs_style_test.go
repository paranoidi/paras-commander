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

func TestJobsProgressStylesCompleted(t *testing.T) {
	t.Parallel()
	th := Default()

	fill, _, labelOnFill, _ := th.JobsProgressStyles("completed")
	if !stylesEqual(fill, th.JobsProgressCompletedFill) {
		t.Fatal("completed fill style mismatch")
	}
	if !stylesEqual(labelOnFill, th.JobsProgressCompletedFill) {
		t.Fatal("completed labelOnFill style mismatch")
	}
	if stylesEqual(fill, th.JobsProgressFill) {
		t.Fatal("completed fill must differ from the active fill")
	}
}

func TestJobsProgressCompletedFallsBackToActive(t *testing.T) {
	t.Parallel()
	data := testTheme(t, "completedfallback", nil, map[string]string{
		"jobs.progress.fill":          `{ fg = "bg", bg = "warning" }`,
		"jobs.progress.label.on_fill": `{ fg = "fg", bold = true }`,
	})
	th, err := parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fg, bg, attrs := th.JobsProgressCompletedFill.Decompose()
	labelFG, _, labelAttrs := th.JobsProgressLabelOnFill.Decompose()
	_, fillBG, _ := th.JobsProgressFill.Decompose()
	if fg != labelFG || attrs != labelAttrs {
		t.Fatal("completed fill should fall back to the active label foreground and attrs")
	}
	if bg != fillBG {
		t.Fatal("completed fill should fall back to the active fill background")
	}
}

func stylesEqual(a, b tcell.Style) bool {
	aFG, aBG, aAttrs := a.Decompose()
	bFG, bBG, bAttrs := b.Decompose()
	return aFG == bFG && aBG == bBG && aAttrs == bAttrs
}
