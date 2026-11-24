package dialog

import (
	"slices"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestIndexTagBrowsePrefillDropsTagsTheIndexLacks(t *testing.T) {
	st := NewIndexTagBrowseDialog([]string{"amber", "birch", "cedar"}, []string{"cedar", "birch", "gone"}, true, true)
	if got := st.Checked(); !slices.Equal(got, []string{"birch", "cedar"}) {
		t.Fatalf("checked = %v", got)
	}
	if !st.Any || !st.Structure || st.Focus != IndexTagsGrid {
		t.Fatalf("any %v structure %v focus %q", st.Any, st.Structure, st.Focus)
	}
	if st.cursorTag() != 1 {
		t.Fatalf("the cursor is on tag %d, want the first checked one", st.cursorTag())
	}
}

func TestIndexTagBrowseTogglesBetweenOnAndOff(t *testing.T) {
	st := NewIndexTagBrowseDialog([]string{"amber", "birch"}, nil, false, false)
	for _, want := range []int{1, 0, 1} {
		st.ToggleCursor()
		if got := len(st.Checked()); got != want {
			t.Fatalf("checked %d tags, want %d", got, want)
		}
	}
}

func TestIndexTagBrowseRadios(t *testing.T) {
	st := NewIndexTagBrowseDialog([]string{"amber"}, nil, false, false)
	st.Apply(dlg.Event{ID: IndexTagBrowseAny})
	st.Apply(dlg.Event{ID: IndexTagBrowseStructure})
	if !st.Any || !st.Structure {
		t.Fatalf("any %v structure %v", st.Any, st.Structure)
	}
	st.Apply(dlg.Event{ID: IndexTagBrowseAll})
	st.Apply(dlg.Event{ID: IndexTagBrowseFlat})
	if st.Any || st.Structure {
		t.Fatalf("any %v structure %v", st.Any, st.Structure)
	}
}

func TestIndexTagBrowseViewHasEveryWidget(t *testing.T) {
	st := NewIndexTagBrowseDialog([]string{"amber", "birch", "cedar"}, nil, false, false)
	spec := IndexTagBrowseDialogView(&st, Layout{Width: 80, Height: 24}, theme.Default())
	for _, id := range []dlg.ID{IndexTagsGrid, IndexTagBrowseAll, IndexTagBrowseAny, IndexTagBrowseFlat, IndexTagBrowseStructure} {
		if _, ok := spec.Item(id); !ok {
			t.Errorf("widget %q missing", id)
		}
	}
	none := NewIndexTagBrowseDialog(nil, nil, false, false)
	none.Focus = IndexTagBrowseAll
	if _, ok := IndexTagBrowseDialogView(&none, Layout{Width: 80, Height: 24}, theme.Default()).Item(IndexTagsGrid); ok {
		t.Error("a grid with no tags should be the label")
	}
}

func TestIndexTagBrowseRendersAtSmallSizes(t *testing.T) {
	st := NewIndexTagBrowseDialog([]string{"amber", "birch", "cedar", "dune", "ember", "fjord", "grove", "heath"}, []string{"dune"}, false, false)
	for _, size := range [][2]int{{80, 24}, {60, 14}, {50, 12}} {
		out := renderIndexTagBrowse(t, st, size[0], size[1])
		for _, want := range []string{"Browse by tags", "OK", "Cancel"} {
			if !strings.Contains(out, want) {
				t.Fatalf("%dx%d: %q missing:\n%s", size[0], size[1], want, out)
			}
		}
	}
}

func TestIndexTagBrowseRadioColumnsLineUp(t *testing.T) {
	st := NewIndexTagBrowseDialog([]string{"amber"}, nil, false, false)
	p, ok := dlg.Place(80, 24, IndexTagBrowseDialogView(&st, Layout{Width: 80, Height: 24}, theme.Default()))
	if !ok {
		t.Fatal("dialog does not fit")
	}
	x := map[dlg.ID]int{}
	for _, it := range p.Items {
		x[it.ID] = it.Rect.X
	}
	if x[IndexTagBrowseAny] == 0 || x[IndexTagBrowseAny] != x[IndexTagBrowseStructure] {
		t.Errorf("right radios at x %d and %d", x[IndexTagBrowseAny], x[IndexTagBrowseStructure])
	}
}
