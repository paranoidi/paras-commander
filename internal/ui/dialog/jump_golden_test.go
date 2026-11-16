package dialog

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for Jump to directory: a disk walk in progress and done, the index form (no
// volume option), match highlights, a capped list, no matches, an error and focus on the depth
// input and OK.

func jumpGoldenState(n int) JumpDialogState {
	st := JumpDialogState{Open: true, Root: "/home/meadow", Query: "lan", QueryCursor: 3, StayOnVolume: true, Focus: JumpQuery}
	for i := 0; i < n; i++ {
		a, b := historyGoldenWords[i%len(historyGoldenWords)], historyGoldenWords[(i*5+3)%len(historyGoldenWords)]
		rel := fmt.Sprintf("%s/%s-%d", a, b, i)
		st.Results = append(st.Results, JumpResult{Path: "/home/meadow/" + rel, Rel: rel, Match: []search.Range{{Start: 0, End: 1}}})
	}
	return st
}

func TestJumpGoldenRender(t *testing.T) {
	goldenBandRows["jump"] = 30
	var cases []simpleGolden
	add := func(name string, w, h int, st JumpDialogState) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawJumpDialog(s, l, st, DialogRenderContext{Styles: asciiDialogStyles(), ScrollbarStyle: uiscrollbar.StyleThumb}, nil, nil)
		}})
	}
	st := jumpGoldenState(12)
	add("default", 100, 30, st)
	scanning := st
	scanning.Scanning, scanning.Scanned, scanning.Depth, scanning.LevelDone, scanning.LevelTotal = true, 1234, 3, 2, 8
	add("scanning", 100, 30, scanning)
	walked := jumpGoldenState(6)
	walked.Query, walked.QueryCursor, walked.Scanned = "", 0, 6
	add("walked_empty_query", 100, 30, walked)
	index := st
	index.Source = JumpIndex
	add("index", 100, 30, index)
	capped := jumpGoldenState(30)
	capped.Truncated = true
	add("capped", 100, 30, capped)
	add("no_matches", 100, 30, jumpGoldenState(0))
	bad := jumpGoldenState(0)
	bad.Err = "open /home/meadow: permission denied"
	add("error", 100, 30, bad)
	opts := st
	opts.IncludeHidden, opts.StayOnVolume, opts.MaxDepth, opts.DepthInput = true, false, 2, "2"
	add("options", 100, 30, opts)
	depth := st
	depth.Focus = JumpDepth
	add("focus-depth", 100, 30, depth)
	ok := st
	ok.Focus = dlg.OK
	add("focus-ok", 100, 30, ok)
	add("narrow60", 60, 24, st)
	runSimpleGoldens(t, "jump", cases)
}
