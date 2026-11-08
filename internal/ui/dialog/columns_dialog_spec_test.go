package dialog_test

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const columnsScreenH = 40

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
		{"file only", nil, []string{"File"}},
		{"meta", []dialog.MetaEntry{{Name: "walnut"}}, []string{"File", "Meta"}},
		{"index", []dialog.MetaEntry{{Name: "fileindex.tags", Label: "Tags", Section: "File Index"}}, []string{"File", "File Index"}},
		{"both", []dialog.MetaEntry{{Name: "walnut"}, {Name: "fileindex.tags", Label: "Tags", Section: "File Index"}}, []string{"File", "Meta", "File Index"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := dialog.ColumnsDialogState{Entries: tc.entries, Checked: make([]bool, len(tc.entries))}
			if got := sectionLabels(dialog.ColumnsDialogView(st, columnsScreenH)); !equalStrings(got, tc.want) {
				t.Fatalf("sections = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestColumnsDialogView_fileColumnsTickFromSet(t *testing.T) {
	st := dialog.ColumnsDialogState{FileColumns: panel.ColumnsPermissions}
	spec := dialog.ColumnsDialogView(st, columnsScreenH)
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

func TestColumnsDialogView_sectionInTwoColumns(t *testing.T) {
	st := dialog.ColumnsDialogState{Entries: []dialog.MetaEntry{{Name: "walnut", Description: "A long description that keeps one column"}}}
	for _, n := range []string{"Status", "Tags", "Copies", "First seen", "Last seen"} {
		st.Entries = append(st.Entries, dialog.MetaEntry{Name: n, Label: n, Section: "File Index"})
	}
	st.Checked = make([]bool, len(st.Entries))
	spec := dialog.ColumnsDialogView(st, columnsScreenH)
	// File header, File rows, Meta header, Meta rows, File Index header, File Index rows.
	if top := spec.Groups[3].Columns[0]; len(top) != 1 || top[0].Kind != dlg.KindCheckbox {
		t.Fatalf("meta section = %+v, want one plain checkbox", top)
	}
	rows := spec.Groups[5].Columns[0]
	if len(rows) != 3 || rows[0].Kind != dlg.KindRow || rows[2].Kind != dlg.KindCheckbox {
		t.Fatalf("section = %+v, want two rows of two then the odd one out", rows)
	}
	// Filled column by column: Status beside First seen, Copies alone.
	if l, r := rows[0].Cells[0], rows[0].Cells[1]; l.ID != dialog.MetaEntryID(1) || r.ID != dialog.MetaEntryID(4) {
		t.Fatalf("first row = %v %v, want Status and First seen", l.Label, r.Label)
	}
	if rows[2].ID != dialog.MetaEntryID(3) {
		t.Fatalf("last row = %v, want Copies", rows[2].Label)
	}

	// Right crosses to the second column, Down walks the first.
	focus := dialog.MetaEntryID(1)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if focus != dialog.MetaEntryID(4) {
		t.Fatalf("Right from Status focuses %q, want First seen", focus)
	}
	focus = dialog.MetaEntryID(2)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if focus != dialog.MetaEntryID(3) {
		t.Fatalf("Down from Tags focuses %q, want Copies", focus)
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
		if got := dialog.ColumnsDialogView(dialog.ColumnsDialogState{Phase: phase}, columnsScreenH).Title; got != want {
			t.Fatalf("phase %d title = %q, want %q", phase, got, want)
		}
	}
}
