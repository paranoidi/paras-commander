package dialog

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the find-in-index dialog: default focus with results and marks, Words mode
// (Only directories greyed), jump mode (no radio rows), the truncated count, an error and the
// empty list.

func indexFindGoldenState(n int) IndexFindDialogState {
	st := IndexFindDialogState{Open: true, Root: "/home/meadow", Query: "lantern", QueryCursor: 7, Focus: IndexFindQuery}
	for i := 0; i < n; i++ {
		a, b := historyGoldenWords[i%len(historyGoldenWords)], historyGoldenWords[(i*5+3)%len(historyGoldenWords)]
		st.Results = append(st.Results, IndexFindResult{Path: fmt.Sprintf("/home/meadow/%s/%s-%d.txt", a, b, i), Rel: fmt.Sprintf("%s/%s-%d.txt", a, b, i), IsDir: i%4 == 0})
	}
	return st
}

func TestIndexFindGoldenRender(t *testing.T) {
	goldenBandRows["index_find"] = 30
	var cases []simpleGolden
	add := func(name string, w, h int, st IndexFindDialogState) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawIndexFindDialog(s, l, st, DialogRenderContext{Styles: asciiDialogStyles(), ScrollbarStyle: uiscrollbar.StyleThumb}, nil)
		}})
	}
	st := indexFindGoldenState(12)
	add("default", 100, 30, st)
	marked := st
	marked.Selected = 2
	marked.Marked = map[string]bool{st.Results[1].Path: true, st.Results[3].Path: true}
	add("marked_cursor2", 100, 30, marked)
	words := st
	words.Words, words.OnlyFiles = true, true
	add("words", 100, 30, words)
	jump := st
	jump.JumpMode, jump.OnlyDirs = true, true
	add("jump", 100, 30, jump)
	trunc := indexFindGoldenState(30)
	trunc.Truncated = true
	add("truncated", 100, 30, trunc)
	searching := st
	searching.Searching = true
	add("searching", 100, 30, searching)
	bad := indexFindGoldenState(0)
	bad.Words, bad.Err = true, "fts5: syntax error near \"\""
	add("error", 100, 30, bad)
	add("empty", 100, 30, indexFindGoldenState(0))
	opts := st
	opts.ShowSearchSelectionsOption, opts.SearchOnlySelections, opts.IncludeHidden = true, true, true
	opts.MaxDepth, opts.DepthInput = 3, "3"
	opts.Marked = map[string]bool{st.Results[1].Path: true, st.Results[2].Path: true}
	opts.MarkedSizes = map[string]int64{st.Results[1].Path: 1500, st.Results[2].Path: 2048}
	add("options_marked_sizes", 100, 30, opts)
	depth := st
	depth.Focus = IndexFindDepth
	add("focus-depth", 100, 30, depth)
	ok := st
	ok.Focus = dlg.OK
	add("focus-ok", 100, 30, ok)
	add("narrow60", 60, 24, st)
	runSimpleGoldens(t, "index_find", cases)
}
