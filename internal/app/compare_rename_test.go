package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	comparectrl "github.com/paranoidi/paras-commander/internal/apphandler/compare"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func writeCompareFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// waitCompareDone pumps the session until its scan finishes.
func waitCompareDone(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		app.pollCompareUpdates(comparectrl.WakePayload{})
		if app.model.CompareSnapshot.Phase == comparepkg.PhaseDone {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("compare did not finish: %+v", app.model.CompareSnapshot)
}

func openCompareOn(t *testing.T, app *App, left, right string) {
	t.Helper()
	if !app.compareCtrl.OpenPaths(pathloc.MustParse(left), pathloc.MustParse(right), false, nil) {
		t.Fatal("OpenPaths refused")
	}
	waitCompareDone(t, app)
}

// compareRowIndex is the filtered-row index of the row with rel on the given side.
func compareRowIndex(t *testing.T, app *App, rel string, secondary bool) int {
	t.Helper()
	i := slices.IndexFunc(app.compareCtrl.FilteredRows(), func(r comparepkg.Row) bool {
		if secondary {
			return r.SecondaryRel == rel
		}
		return r.PrimaryRel == rel
	})
	if i < 0 {
		t.Fatalf("no row for %q in %+v", rel, app.compareCtrl.FilteredRows())
	}
	return i
}

func pressKey(t *testing.T, app *App, key tcell.Key, mod tcell.ModMask) {
	t.Helper()
	if quit, _ := app.handleKey(tcell.NewEventKey(key, 0, mod)); quit {
		t.Fatal("unexpected quit")
	}
}

func setRenameField(app *App, name string) {
	f := &app.model.FileDialog.Fields[0]
	f.Value, f.Cursor, f.PrefillPending = name, len([]rune(name)), false
}

func TestCompareFooterShowsRenameWithMassRenameShiftWord(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	keys := compareViewFooterKeys(bundle.Global, bundle.Compare, comparepkg.FilterAll, true)
	i := slices.IndexFunc(keys, func(fk menu.FunctionKey) bool { return fk.ActionID == keymap.ActionCompareRename })
	if i < 0 {
		t.Fatalf("footer has no rename entry: %+v", keys)
	}
	fk := keys[i]
	if fk.KeyLabel != "F6" || fk.Hint != "Rename" || fk.HintShiftPrefix != "Mass" || fk.ShiftKeyLabel != "S-F6" {
		t.Fatalf("rename entry = %+v", fk)
	}
}

func TestCompareFooterSplitsRenameEntriesWhenRebound(t *testing.T) {
	m, err := keymap.Build(map[string][]string{
		keymap.ActionCompareRename:     {"F6"},
		keymap.ActionCompareMassRename: {"M-m"},
	})
	if err != nil {
		t.Fatal(err)
	}
	keys := appendShiftTwinFooterKey(nil, m, keymap.ActionCompareRename, keymap.ActionCompareMassRename, "Rename", "Mass", "Mass rename")
	if len(keys) != 2 || keys[0].HintShiftPrefix != "" || keys[1].Hint != "Mass rename" || keys[1].ActionID != keymap.ActionCompareMassRename {
		t.Fatalf("footer = %+v", keys)
	}
}

func TestCompareF6KeepsBrowserMoveBinding(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	f6 := tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone)
	if id, ok := bundle.Global.Lookup(f6); !ok || id != keymap.ActionMove {
		t.Fatalf("global F6 = %q %v", id, ok)
	}
	if id, ok := bundle.Compare.Lookup(f6); !ok || id != keymap.ActionCompareRename {
		t.Fatalf("compare F6 = %q %v", id, ok)
	}
	sf6 := tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModShift)
	if id, ok := bundle.Compare.Lookup(sf6); !ok || id != keymap.ActionCompareMassRename {
		t.Fatalf("compare S-F6 = %q %v", id, ok)
	}
}

// F6 renames the nested file of the focused column, which need not be the active panel, and the
// results rescan so the rename shows and the pair reclassifies.
func TestCompareF6RenamesFocusedColumnFileAndRescans(t *testing.T) {
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, t.TempDir())
	left, right := t.TempDir(), t.TempDir()
	writeCompareFile(t, left, "orchard/lantern.txt", "same bytes")
	old := writeCompareFile(t, right, "orchard/lantern.txt", "same bytes")
	app.model.ActivePanel = ui.PrimaryPanel
	openCompareOn(t, app, left, right)
	app.model.CompareView.FocusColumn = ui.CompareColumnSecondary

	pressKey(t, app, tcell.KeyF6, tcell.ModNone)
	fd := &app.model.FileDialog
	if !fd.Open || fd.DialogType != dialog.FileDialogRename || fd.RenameSource != old {
		t.Fatalf("rename dialog = %+v", *fd)
	}
	setRenameField(app, "harbor.txt")
	app.dialogCtrl.ExecuteFileDialog()

	renamed := filepath.Join(right, "orchard", "harbor.txt")
	if _, err := os.Stat(renamed); err != nil {
		t.Fatalf("renamed file: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old file still there: %v", err)
	}
	if app.model.FileDialog.Open {
		t.Fatal("dialog still open")
	}
	waitCompareDone(t, app)
	if app.model.ViewMode != ui.ViewCompare || app.model.CompareView.FocusColumn != ui.CompareColumnSecondary {
		t.Fatalf("view/focus not kept: %v %v", app.model.ViewMode, app.model.CompareView.FocusColumn)
	}
	row := app.compareCtrl.FilteredRows()[compareRowIndex(t, app, "orchard/harbor.txt", true)]
	if row.Kind != comparepkg.KindRelocated {
		t.Fatalf("row after rename = %+v, want relocated", row)
	}
}

// A selected file renamed from its row keeps its selection under the new name, even though it is
// not in the panel's listing.
func TestCompareF6MovesSelectionToNewName(t *testing.T) {
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, t.TempDir())
	left, right := t.TempDir(), t.TempDir()
	old := writeCompareFile(t, left, "orchard/lantern.txt", "only here")
	openCompareOn(t, app, left, right)
	app.panelByID(ui.PrimaryPanel).AddSelection(old)

	pressKey(t, app, tcell.KeyF6, tcell.ModNone)
	setRenameField(app, "harbor.txt")
	app.dialogCtrl.ExecuteFileDialog()

	p := app.panelByID(ui.PrimaryPanel)
	if p.IsSelectedPath(old) || !p.IsSelectedPath(filepath.Join(left, "orchard", "harbor.txt")) {
		t.Fatalf("selection = %v", p.SelectedPaths)
	}
}

func applyMassRename(t *testing.T, app *App, find, replace string) {
	t.Helper()
	fd := &app.model.FileDialog
	if !fd.Open || fd.DialogType != dialog.FileDialogMassRename {
		t.Fatalf("mass rename dialog = %+v", *fd)
	}
	fd.Fields[0].Value, fd.Fields[1].Value = find, replace
	app.dialogCtrl.ExecuteFileDialog()
}

// S-F6 takes the focused column's panel selection and clears that panel's selection, leaving the
// other panel's alone.
func TestCompareMassRenameUsesFocusedPanelSelection(t *testing.T) {
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, t.TempDir())
	left, right := t.TempDir(), t.TempDir()
	leftOnly := writeCompareFile(t, left, "meadow/thistle.txt", "left")
	a := writeCompareFile(t, right, "orchard/lantern.txt", "first")
	b := writeCompareFile(t, right, "quarry/lantern-two.txt", "second")
	app.model.ActivePanel = ui.PrimaryPanel
	openCompareOn(t, app, left, right)
	app.model.CompareView.FocusColumn = ui.CompareColumnSecondary
	focused, other := app.panelByID(ui.SecondaryPanel), app.panelByID(ui.PrimaryPanel)
	focused.AddSelection(a)
	focused.AddSelection(b)
	other.AddSelection(leftOnly)

	pressKey(t, app, tcell.KeyF6, tcell.ModShift)
	if n := len(app.model.FileDialog.MassRenameSources); n != 2 {
		t.Fatalf("sources = %+v", app.model.FileDialog.MassRenameSources)
	}
	applyMassRename(t, app, "lantern", "beacon")

	for _, p := range []string{filepath.Join(right, "orchard", "beacon.txt"), filepath.Join(right, "quarry", "beacon-two.txt")} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("renamed: %v", err)
		}
	}
	if len(focused.SelectedPaths) != 0 {
		t.Fatalf("focused selection = %v", focused.SelectedPaths)
	}
	if !other.IsSelectedPath(leftOnly) {
		t.Fatalf("other panel's selection was touched: %v", other.SelectedPaths)
	}
	waitCompareDone(t, app)
	compareRowIndex(t, app, "orchard/beacon.txt", true)
}

// Without a selection S-F6 acts on the cursor row's file, not on the panel's own cursor entry.
func TestCompareMassRenameWithoutSelectionUsesCursorRow(t *testing.T) {
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, t.TempDir())
	left, right := t.TempDir(), t.TempDir()
	path := writeCompareFile(t, left, "orchard/lantern.txt", "only here")
	openCompareOn(t, app, left, right)

	pressKey(t, app, tcell.KeyF6, tcell.ModShift)
	src := app.model.FileDialog.MassRenameSources
	if len(src) != 1 || src[0].Path != path || src[0].Name != "lantern.txt" {
		t.Fatalf("sources = %+v", src)
	}
}

func TestCompareRenameOnEmptySideShowsToast(t *testing.T) {
	for _, tc := range []struct {
		name string
		mod  tcell.ModMask
	}{{"rename", tcell.ModNone}, {"mass rename", tcell.ModShift}} {
		t.Run(tc.name, func(t *testing.T) {
			screen := newScreen(t, 100, 30)
			app := newApp(t, screen, t.TempDir())
			left, right := t.TempDir(), t.TempDir()
			writeCompareFile(t, left, "lantern.txt", "only here")
			openCompareOn(t, app, left, right)
			app.model.CompareView.FocusColumn = ui.CompareColumnSecondary

			pressKey(t, app, tcell.KeyF6, tc.mod)
			if app.model.FileDialog.Open {
				t.Fatalf("dialog opened: %+v", app.model.FileDialog)
			}
			if got := app.model.Message; got != compareNothingToRename {
				t.Fatalf("message = %q", got)
			}
		})
	}
}


// Apply rescans while the dialog stays open, and the batch stays selected under its new names.
func TestCompareMassRenameApplyKeepsDialogOpenThroughRescan(t *testing.T) {
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, t.TempDir())
	left, right := t.TempDir(), t.TempDir()
	path := writeCompareFile(t, left, "orchard/lantern.txt", "only here")
	openCompareOn(t, app, left, right)
	p := app.panelByID(ui.PrimaryPanel)
	p.AddSelection(path)

	pressKey(t, app, tcell.KeyF6, tcell.ModShift)
	fd := &app.model.FileDialog
	fd.Fields[0].Value, fd.Fields[1].Value = "lantern", "beacon"
	app.dialogCtrl.ApplyMassRenameKeepOpen()

	renamed := filepath.Join(left, "orchard", "beacon.txt")
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogMassRename {
		t.Fatalf("dialog closed by the rescan: %+v", app.model.FileDialog)
	}
	if got := app.model.FileDialog.MassRenameSources; len(got) != 1 || got[0].Path != renamed {
		t.Fatalf("sources = %+v", got)
	}
	if !p.IsSelectedPath(renamed) || p.IsSelectedPath(path) {
		t.Fatalf("selection = %v", p.SelectedPaths)
	}
	waitCompareDone(t, app)
	compareRowIndex(t, app, "orchard/beacon.txt", false)
	app.dialogCtrl.CloseFileDialog()
	if len(p.SelectedPaths) != 0 {
		t.Fatalf("selection after close = %v", p.SelectedPaths)
	}
}

// With swapped panes the left column belongs to the Secondary panel.
func TestCompareMassRenameFollowsSwappedPanes(t *testing.T) {
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, t.TempDir())
	left, right := t.TempDir(), t.TempDir()
	path := writeCompareFile(t, left, "orchard/lantern.txt", "only here")
	openCompareOn(t, app, left, right)
	app.model.CompareView.SwapPanels = true
	secondary := app.panelByID(ui.SecondaryPanel)
	secondary.AddSelection(path)

	pressKey(t, app, tcell.KeyF6, tcell.ModShift)
	applyMassRename(t, app, "lantern", "beacon")

	if _, err := os.Stat(filepath.Join(left, "orchard", "beacon.txt")); err != nil {
		t.Fatal(err)
	}
	if len(secondary.SelectedPaths) != 0 {
		t.Fatalf("selection = %v", secondary.SelectedPaths)
	}
}
