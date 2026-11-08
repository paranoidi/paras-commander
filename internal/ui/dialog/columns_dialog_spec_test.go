package dialog_test

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const columnsScreenW, columnsScreenH = 120, 40

func TestColumnsDialogShortcuts_fileColumnsArePinned(t *testing.T) {
	got := dialog.ColumnsDialogShortcuts([]dialog.MetaEntry{{Name: "size-on-disk"}})
	for i, spec := range panel.Columns() {
		if got[i] != spec.Mnemonic {
			t.Fatalf("%s mnemonic = %q, want %q", spec.Name, got[i], spec.Mnemonic)
		}
	}
	if entry := got[len(panel.Columns())]; entry == 's' || entry == 'o' || entry == 'c' {
		t.Fatalf("entry mnemonic = %q, must avoid the file columns and buttons", entry)
	}
}

func TestColumnsDialogShortcuts_unique(t *testing.T) {
	got := dialog.ColumnsDialogShortcuts([]dialog.MetaEntry{{Name: "juniper"}, {Name: "juniper-two"}, {Name: "kestrel"}})
	seen := map[rune]bool{}
	for _, r := range got {
		if r != 0 && seen[r] {
			t.Fatalf("duplicate mnemonic %q in %q", r, got)
		}
		seen[r] = true
	}
}

// sectionLabels lists the labelled separators of a spec, in order.
func sectionLabels(spec dlg.Spec) []string {
	var labels []string
	for _, g := range spec.Groups {
		for _, it := range g.Columns[0] {
			if it.Kind == dlg.KindSeparator && it.Label != "" {
				labels = append(labels, it.Label)
			}
		}
	}
	return labels
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestColumnsDialogView_sectionsHideWhenEmpty(t *testing.T) {
	cases := []struct {
		name    string
		entries []dialog.MetaEntry
		want    []string
	}{
		{"file only", nil, nil},
		{"meta", []dialog.MetaEntry{{Name: "walnut"}}, []string{"Meta"}},
		{"index", []dialog.MetaEntry{{Name: "fileindex.tags", Label: "Tags", Section: "File Index"}}, []string{"File Index"}},
		{"both", []dialog.MetaEntry{{Name: "walnut"}, {Name: "fileindex.tags", Label: "Tags", Section: "File Index"}}, []string{"Meta", "File Index"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := dialog.ColumnsDialogState{Entries: tc.entries, Checked: make([]bool, len(tc.entries))}
			if got := sectionLabels(dialog.ColumnsDialogView(st, columnsScreenW, columnsScreenH)); !equalStrings(got, tc.want) {
				t.Fatalf("sections = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestColumnsDialogView_fileColumnsTickFromSet(t *testing.T) {
	st := dialog.ColumnsDialogState{FileColumns: panel.ColumnsPermissions}
	spec := dialog.ColumnsDialogView(st, columnsScreenW, columnsScreenH)
	on := map[dlg.ID]bool{}
	var walk func(items []dlg.Item)
	walk = func(items []dlg.Item) {
		for _, it := range items {
			switch it.Kind {
			case dlg.KindRow:
				walk(it.Cells)
			case dlg.KindCheckbox:
				on[it.ID] = it.On
			}
		}
	}
	for _, g := range spec.Groups {
		walk(g.Columns[0])
	}
	for i, c := range panel.Columns() {
		want := panel.ColumnsPermissions.Has(c.ID)
		got, found := on[dialog.ColumnsFileID(i)]
		if !found || got != want {
			t.Fatalf("%s: on = %v (found %v), want %v", c.Name, got, found, want)
		}
	}
}

func TestColumnsDialogApply_togglesFileColumnAndEntry(t *testing.T) {
	st := dialog.ColumnsDialogState{
		FileColumns: panel.ColumnsBrief,
		Entries:     []dialog.MetaEntry{{Name: "walnut"}},
		Checked:     []bool{false},
	}
	st.Apply(dlg.Event{ID: dialog.ColumnsFileID(0)})
	if st.FileColumns != 0 {
		t.Fatalf("size not toggled off: %v", st.FileColumns.Names())
	}
	st.Apply(dlg.Event{ID: dialog.MetaEntryID(0)})
	if !st.Checked[0] {
		t.Fatal("entry not toggled on")
	}
}

// gridCells returns the placed x of every checkbox by ID.
func gridCells(t *testing.T, spec dlg.Spec, w, h int) map[dlg.ID]int {
	t.Helper()
	p, ok := dlg.Place(w, h, spec)
	if !ok {
		t.Fatalf("dialog does not place at %dx%d: width %d min %d", w, h, spec.Width, spec.MinWidth)
	}
	xs := map[dlg.ID]int{}
	for _, g := range spec.Groups {
		for _, it := range g.Columns[0] {
			for _, c := range it.Cells {
				placed, found := p.Item(c.ID)
				if !found || placed.Rect.Width == 0 {
					t.Fatalf("%s not drawn", c.ID)
				}
				xs[c.ID] = placed.Rect.X
			}
		}
	}
	return xs
}

func gridState() dialog.ColumnsDialogState {
	st := dialog.ColumnsDialogState{Entries: []dialog.MetaEntry{
		{Name: "wordcount", Description: "Word count"},
		{Name: "juniper"},
	}}
	for _, n := range []string{"Status", "Tags", "Copies", "First seen", "Last seen"} {
		st.Entries = append(st.Entries, dialog.MetaEntry{Name: n, Label: n, Section: "File Index"})
	}
	st.Checked = make([]bool, len(st.Entries))
	return st
}

func TestColumnsDialogView_threeColumnsAlignAcrossSections(t *testing.T) {
	spec := dialog.ColumnsDialogView(gridState(), columnsScreenW, columnsScreenH)
	xs := gridCells(t, spec, columnsScreenW, columnsScreenH)
	// File fills column by column: three rows of three.
	col0, col1, col2 := xs[dialog.ColumnsFileID(0)], xs[dialog.ColumnsFileID(3)], xs[dialog.ColumnsFileID(6)]
	if col0 >= col1 || col1 >= col2 {
		t.Fatalf("file columns at %d %d %d", col0, col1, col2)
	}
	for id, want := range map[dlg.ID]int{
		dialog.ColumnsFileID(1): col0,
		dialog.ColumnsFileID(4): col1,
		dialog.ColumnsFileID(8): col2,
		dialog.MetaEntryID(0):   col0, // Meta: wordcount, juniper side by side
		dialog.MetaEntryID(1):   col1,
		dialog.MetaEntryID(2):   col0, // File Index: Status, Tags / First seen, Last seen / Copies is alone
		dialog.MetaEntryID(4):   col1,
		dialog.MetaEntryID(6):   col2,
	} {
		if xs[id] != want {
			t.Fatalf("%s at x=%d, want %d", id, xs[id], want)
		}
	}
}

func TestColumnsDialogView_narrowScreenDropsColumnsAndDrawsEveryBox(t *testing.T) {
	st := gridState()
	for _, w := range []int{60, 44, 32} {
		spec := dialog.ColumnsDialogView(st, w, columnsScreenH)
		xs := gridCells(t, spec, w, columnsScreenH)
		if len(xs) != len(panel.Columns())+len(st.Entries) {
			t.Fatalf("width %d: %d boxes", w, len(xs))
		}
	}
}

func TestColumnsDialogView_labelsAreClippedNotWidenedForever(t *testing.T) {
	st := dialog.ColumnsDialogState{Entries: []dialog.MetaEntry{{Name: "a-meta-column-name-far-too-long-for-any-grid"}}, Checked: []bool{false}}
	spec := dialog.ColumnsDialogView(st, columnsScreenW, columnsScreenH)
	if spec.Width > 90 {
		t.Fatalf("dialog width = %d", spec.Width)
	}
}

func TestColumnsDialogView_descriptionFollowsFocusAndKeepsItsRow(t *testing.T) {
	st := gridState()
	label := func(st dialog.ColumnsDialogState) (string, bool) {
		spec := dialog.ColumnsDialogView(st, columnsScreenW, columnsScreenH)
		items := spec.Groups[len(spec.Groups)-1].Columns[0]
		if len(items) != 2 || items[1].Kind != dlg.KindLabel {
			return "", false
		}
		return items[1].Label, true
	}
	st.Focus = dialog.MetaEntryID(0)
	if got, ok := label(st); !ok || got != "Word count" {
		t.Fatalf("description = %q, row %v", got, ok)
	}
	st.Focus = dialog.ColumnsFileID(0)
	if got, ok := label(st); !ok || got != "" {
		t.Fatalf("file column: description = %q, row %v", got, ok)
	}
	st.Entries = []dialog.MetaEntry{{Name: "juniper"}}
	st.Checked = []bool{false}
	if _, ok := label(st); ok {
		t.Fatal("no entry has a description: no row")
	}
}

func TestColumnsDialogView_upDownStayInColumn(t *testing.T) {
	spec := dialog.ColumnsDialogView(gridState(), columnsScreenW, columnsScreenH)
	focus := dialog.ColumnsFileID(3)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if focus != dialog.ColumnsFileID(4) {
		t.Fatalf("Down from the second column focuses %q", focus)
	}
	focus = dialog.ColumnsFileID(0)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if focus != dialog.ColumnsFileID(3) {
		t.Fatalf("Right from Size focuses %q", focus)
	}
}

func TestColumnsProfileLine_marks(t *testing.T) {
	p := columnprofile.Profile{Name: "brief", Description: "size only", Builtin: true}
	if got := dialog.ColumnsProfileLine(p, "brief"); got != "brief — size only (built-in) (default)" {
		t.Fatalf("line = %q", got)
	}
	if got := dialog.ColumnsProfileLine(columnprofile.Profile{Name: "orchard"}, "brief"); got != "orchard" {
		t.Fatalf("line = %q", got)
	}
}

func TestColumnsDialogView_phaseTitles(t *testing.T) {
	for phase, want := range map[dialog.ColumnsPhase]string{
		dialog.ColumnsPhaseMain:            "Columns",
		dialog.ColumnsPhaseLoadPicker:      "Load profile",
		dialog.ColumnsPhaseOverwritePicker: "Overwrite profile",
		dialog.ColumnsPhaseSavePrompt:      "Save profile",
	} {
		if got := dialog.ColumnsDialogView(dialog.ColumnsDialogState{Phase: phase}, columnsScreenW, columnsScreenH).Title; got != want {
			t.Fatalf("phase %d title = %q, want %q", phase, got, want)
		}
	}
}
