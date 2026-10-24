package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

const compareMergeGoldenHome = "/home/alice"

// compareMergeGoldenState builds a merge dialog. paths: shared (a common prefix is shown), plain
// (nothing in common), here (one side is the shared prefix itself), long (paths wider than the
// dialog). opts: letters r (merge toward the right side), m (Missing files), f (Modified files),
// d (Move). focus: left, right, missing, modified, copy, move, ok, cancel.
func compareMergeGoldenState(paths, opts, focus string) CompareMergeDialogState {
	st := CompareMergeDialogState{
		Open:         true,
		Direction:    comparepkg.MergeTowardPrimary,
		CopyMissing:  strings.Contains(opts, "m"),
		CopyModified: strings.Contains(opts, "f"),
		MoveMode:     strings.Contains(opts, "d"),
		PreviewText:  "Preview: 12 to copy (3.4 MiB)",
	}
	if strings.Contains(opts, "r") {
		st.Direction = comparepkg.MergeTowardSecondary
	}
	if st.MoveMode {
		st.PreviewText = "Preview: 12 to move (3.4 MiB)"
	}
	switch paths {
	case "shared":
		st.PrimaryPath, st.SecondaryPath = "/home/alice/projects/harbor/orchard/meadow", "/home/alice/projects/harbor/willow"
	case "plain":
		st.PrimaryPath, st.SecondaryPath = "/srv/orchard/meadow", "/mnt/willow/lantern"
	case "here":
		st.PrimaryPath, st.SecondaryPath = "/home/alice/projects/harbor", "/home/alice/projects/harbor/orchard"
	case "long":
		st.PrimaryPath = "/home/alice/projects/" + strings.Repeat("harbor-orchard-meadow/", 4) + "lantern"
		st.SecondaryPath = "/home/alice/projects/" + strings.Repeat("willow-thicket-anchor/", 4) + "marble"
		st.PreviewText = "Preview: 12345 to copy (3.4 MiB) which is a very long preview line that does not fit the dialog at all"
	}
	setCompareMergeGoldenFocus(&st, focus)
	return st
}

func TestCompareMergeGoldenRender(t *testing.T) {
	goldenBandRows["compare_merge"] = 24
	var cases []simpleGolden
	add := func(w, h int, paths, opts, focus string) {
		st := compareMergeGoldenState(paths, opts, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s_opts-%s_focus-%s", w, h, paths, opts, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawCompareMergeDialog(s, l, st, asciiDialogStyles(), compareMergeGoldenHome)
		}})
	}
	for _, focus := range []string{"left", "right", "missing", "modified", "copy", "move", "ok", "cancel"} {
		add(80, 30, "shared", "mf", focus)
	}
	add(80, 30, "shared", "rmf", "left")
	add(80, 30, "shared", "rf", "missing")
	add(80, 30, "shared", "m", "modified")
	add(80, 30, "shared", "-", "copy")
	add(80, 30, "shared", "rmfd", "move")
	add(80, 30, "shared", "mfd", "right")
	add(80, 30, "plain", "mf", "left")
	add(80, 30, "plain", "rd", "ok")
	add(80, 30, "here", "mf", "right")
	add(80, 30, "long", "mf", "left")
	add(80, 30, "long", "rmfd", "cancel")
	add(100, 40, "shared", "mf", "left")
	add(60, 30, "shared", "mf", "left") // narrow: width = W-4
	add(60, 30, "long", "rmf", "ok")
	add(54, 30, "plain", "mf", "missing") // the 52 floor
	add(80, 22, "shared", "mf", "left")   // short screen
	runSimpleGoldens(t, "compare_merge", cases)
}
