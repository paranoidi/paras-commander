package dedup

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSelectRangeMarksFilesBetweenAnchorAndCursor(t *testing.T) {
	root := pathloc.MustParse("/scan")
	files := []string{"lantern.txt", "meadow.txt", "quartz.txt", "harbor.txt"}
	var fs = dedupDoneSnapshot(root)
	for _, n := range files {
		fs.Groups[0].Files = append(fs.Groups[0].Files, dedupFile(n))
	}
	h, model := dedupHandlerWithView(t, fs, ui.DedupViewState{
		TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{},
	})
	var fileRows []int
	for i, r := range model.DedupList {
		if r.Value.Kind == ui.DedupRowFile {
			fileRows = append(fileRows, i)
		}
	}
	if len(fileRows) < 4 {
		t.Fatalf("want 4 visible file rows, got %d", len(fileRows))
	}
	st := &model.DedupView
	st.Main.Selected = fileRows[0]
	if res, _ := h.SelectRange(); res != primitive.RangeStarted || st.RangeAnchor == "" {
		t.Fatalf("first press = %v anchor %q, want RangeStarted with anchor", res, st.RangeAnchor)
	}
	st.Main.Selected = fileRows[2]
	res, added := h.SelectRange()
	if res != primitive.RangeDone || added != 3 || st.MarkedCount != 3 || st.RangeAnchor != "" {
		t.Fatalf("second press = %v added %d marked %d anchor %q, want Done/3/3/empty", res, added, st.MarkedCount, st.RangeAnchor)
	}
	if st.Marked[model.DedupList[fileRows[3]].Value.AbsKey] {
		t.Fatal("row past the cursor must stay unmarked")
	}
}
