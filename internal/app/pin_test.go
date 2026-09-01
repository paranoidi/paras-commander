package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestPinToggleAddsAndRemoves(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "alpha.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if added := app.pinCtrl.Toggle(p, false); !added {
		t.Fatal("first toggle should add the pin")
	}
	if len(app.model.PinnedItems) != 1 || app.model.PinnedItems[0].Path != filepath.Clean(p) {
		t.Fatalf("PinnedItems after add = %+v", app.model.PinnedItems)
	}
	if added := app.pinCtrl.Toggle(p, false); added {
		t.Fatal("second toggle on the same path should remove, not add")
	}
	if len(app.model.PinnedItems) != 0 {
		t.Fatalf("PinnedItems after remove = %+v, want empty", app.model.PinnedItems)
	}
}

func TestPinTogglePrependsMostRecentFirst(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	for _, p := range []string{aPath, bPath} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.pinCtrl.Toggle(aPath, false)
	app.pinCtrl.Toggle(bPath, false)
	if len(app.model.PinnedItems) != 2 {
		t.Fatalf("PinnedItems count = %d, want 2", len(app.model.PinnedItems))
	}
	if app.model.PinnedItems[0].Path != filepath.Clean(bPath) {
		t.Fatalf("PinnedItems[0] = %q, want most-recently-pinned %q", app.model.PinnedItems[0].Path, bPath)
	}
	if app.model.PinnedItems[1].Path != filepath.Clean(aPath) {
		t.Fatalf("PinnedItems[1] = %q, want %q", app.model.PinnedItems[1].Path, aPath)
	}
}

func TestPinToggleDoesNotCreateDuplicatesOnRepeat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "dupe.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	for i := 0; i < 3; i++ {
		app.pinCtrl.Toggle(p, false)
	}
	// Odd number of toggles: pinned once, so exactly one entry.
	if len(app.model.PinnedItems) != 1 {
		t.Fatalf("PinnedItems after 3 toggles = %+v, want 1 entry (odd toggle count)", app.model.PinnedItems)
	}
	app.pinCtrl.Toggle(p, false)
	if len(app.model.PinnedItems) != 0 {
		t.Fatalf("PinnedItems after 4 toggles = %+v, want 0 entries (even toggle count)", app.model.PinnedItems)
	}
}

func TestPinToggleActivePanelCursorPinsCurrentEntry(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "current.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	entry, ok := app.activePanel().CurrentEntry()
	if !ok {
		t.Fatal("expected a current entry in the freshly loaded panel")
	}
	app.pinCtrl.ToggleActivePanelCursor()
	if len(app.model.PinnedItems) != 1 {
		t.Fatalf("PinnedItems = %+v, want 1 entry", app.model.PinnedItems)
	}
	if app.model.PinnedItems[0].Path != filepath.Clean(entry.Path) {
		t.Fatalf("pinned path = %q, want %q", app.model.PinnedItems[0].Path, entry.Path)
	}
	if app.model.PinnedItems[0].IsDir != entry.IsDir() {
		t.Fatalf("pinned IsDir = %v, want %v", app.model.PinnedItems[0].IsDir, entry.IsDir())
	}
}

func TestOpenPinDialogRecomputesPathMissing(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "gone.txt")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{
		{Path: present, IsDir: false},
		{Path: missing, IsDir: false},
	}
	app.pinCtrl.OpenDialog()
	if !app.model.PinDialog.Open {
		t.Fatal("expected Pin dialog to open with a non-empty pin list")
	}
	if app.model.PinnedItems[0].PathMissing {
		t.Fatalf("PinnedItems[0] (%q) should exist", present)
	}
	if !app.model.PinnedItems[1].PathMissing {
		t.Fatalf("PinnedItems[1] (%q) should be flagged missing", missing)
	}
}

func TestOpenPinDialogEmptyListShowsMessage(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.pinCtrl.OpenDialog()
	if app.model.PinDialog.Open {
		t.Fatal("Pin dialog should not open with an empty pin list")
	}
}

func TestPinDialogQueryNarrowsRanked(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{
		{Path: filepath.Join(dir, "apple.txt")},
		{Path: filepath.Join(dir, "banana.txt")},
		{Path: filepath.Join(dir, "applesauce.txt")},
	}
	app.pinCtrl.OpenDialog()
	if len(app.model.PinDialog.Ranked) != 3 {
		t.Fatalf("Ranked with empty query = %d, want 3", len(app.model.PinDialog.Ranked))
	}

	app.model.PinDialog.Query = "apple"
	app.pinCtrl.SyncDialogRanks()
	if len(app.model.PinDialog.Ranked) != 2 {
		t.Fatalf("Ranked with query %q = %d, want 2", app.model.PinDialog.Query, len(app.model.PinDialog.Ranked))
	}
	for _, idx := range app.model.PinDialog.Ranked {
		if idx < 0 || idx >= len(app.model.PinnedItems) {
			t.Fatalf("Ranked index %d out of range for %d pinned items", idx, len(app.model.PinnedItems))
		}
	}
}

func TestPinDialogRemoveSelectedResyncsRanks(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{
		{Path: filepath.Join(dir, "one.txt")},
		{Path: filepath.Join(dir, "two.txt")},
		{Path: filepath.Join(dir, "three.txt")},
	}
	app.pinCtrl.OpenDialog()
	app.model.PinDialog.Selected = 1 // "two.txt" (Ranked is identity order for an empty query)

	app.pinCtrl.RemoveSelected()

	if len(app.model.PinnedItems) != 2 {
		t.Fatalf("PinnedItems after remove = %d, want 2", len(app.model.PinnedItems))
	}
	if len(app.model.PinDialog.Ranked) != 2 {
		t.Fatalf("Ranked after remove = %d, want 2", len(app.model.PinDialog.Ranked))
	}
	for _, idx := range app.model.PinDialog.Ranked {
		if idx < 0 || idx >= len(app.model.PinnedItems) {
			t.Fatalf("stale Ranked index %d after remove (PinnedItems has %d entries)", idx, len(app.model.PinnedItems))
		}
	}
	for _, it := range app.model.PinnedItems {
		if filepath.Base(it.Path) == "two.txt" {
			t.Fatal("two.txt should have been removed")
		}
	}
}

func TestPinDialogRemoveAllClearsAndCloses(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{
		{Path: filepath.Join(dir, "one.txt")},
		{Path: filepath.Join(dir, "two.txt")},
		{Path: filepath.Join(dir, "three.txt")},
	}
	app.pinCtrl.OpenDialog()

	app.pinCtrl.RemoveAll()

	if len(app.model.PinnedItems) != 0 {
		t.Fatalf("PinnedItems after RemoveAll = %d, want 0", len(app.model.PinnedItems))
	}
	if app.model.PinDialog.Open {
		t.Fatal("Pin dialog should be closed after RemoveAll")
	}
}

func TestPinDialogRemoveAllNoopOnEmptyList(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.pinCtrl.RemoveAll()

	if len(app.model.PinnedItems) != 0 {
		t.Fatalf("PinnedItems = %d, want 0", len(app.model.PinnedItems))
	}
	if app.model.PinDialog.Open {
		t.Fatal("RemoveAll should not open the Pin dialog on an already-empty list")
	}
}

func TestPinDialogOpenSelectedReturnsFalseOutOfRange(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{{Path: filepath.Join(dir, "only.txt")}}
	app.pinCtrl.OpenDialog()
	app.model.PinDialog.Selected = 5 // out of range for a 1-item Ranked list

	if app.pinCtrl.OpenSelected(ui.PrimaryPanel) {
		t.Fatal("expected pinDialogOpenSelected to return false for an out-of-range selection")
	}
}

func TestPinDialogOpenSelectedReturnsFalseOnNavigateError(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// An empty Path makes pathloc.Parse fail synchronously inside navigatePanelToDirectory
	// (real pins never carry an empty path; this exercises the error-return branch directly).
	app.model.PinnedItems = []ui.PinnedItem{{Path: "", IsDir: true}}
	app.pinCtrl.OpenDialog()
	app.model.PinDialog.Selected = 0

	if app.pinCtrl.OpenSelected(ui.PrimaryPanel) {
		t.Fatal("expected pinDialogOpenSelected to return false on a navigation error")
	}
	if !app.model.PinDialog.Open {
		t.Fatal("dialog should stay open on a navigation error")
	}
}

func TestPinDialogViewSelectedRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{{Path: sub, IsDir: true}}
	app.pinCtrl.OpenDialog()
	app.model.PinDialog.Selected = 0

	app.pinCtrl.ViewSelected()

	if !app.model.PinDialog.Open {
		t.Fatal("dialog should stay open when the selected pin is a directory")
	}
	if app.model.Message != "View: not a file" {
		t.Fatalf("status message = %q, want %q", app.model.Message, "View: not a file")
	}
}

func TestActivatePinSelectionClosesOnlyOnSuccess(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{{Path: "", IsDir: true}}
	app.pinCtrl.OpenDialog()
	app.model.ActivePanel = ui.PrimaryPanel
	app.pinCtrl.ActivateSelection()
	if !app.model.PinDialog.Open {
		t.Fatal("dialog should stay open after a failed activation")
	}

	app.model.PinnedItems = []ui.PinnedItem{{Path: sub, IsDir: true}}
	app.pinCtrl.SyncDialogRanks()
	app.model.PinDialog.Selected = 0
	app.pinCtrl.ActivateSelection()
	if app.model.PinDialog.Open {
		t.Fatal("dialog should close after a successful activation")
	}
	applyNextInterruptEvent(t, app, screen) // async load triggered by the navigation
	if got := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String()); got != filepath.Clean(sub) {
		t.Fatalf("active panel path = %q, want %q", got, sub)
	}
}

// TestPinDialogViewSelectedRestoresDialogAfterPreviewCloses covers the F3-from-Pin round trip:
// opening the fullscreen preview from the Pin dialog hides the dialog (not closePinDialog —
// its state must survive), and closing the preview later reopens it with the exact same
// Query/Selected/ListScroll it had before.
func TestPinDialogViewSelectedRestoresDialogAfterPreviewCloses(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// A large pin list with the previewable target last, so a real ListScroll is needed to
	// keep the selection visible - exercising restore of more than just Query/Selected.
	items := make([]ui.PinnedItem, 0, 15)
	for i := 0; i < 14; i++ {
		items = append(items, ui.PinnedItem{Path: fmt.Sprintf("/pin/item-%d", i)})
	}
	items = append(items, ui.PinnedItem{Path: target})
	app.model.PinnedItems = items
	app.pinCtrl.OpenDialog()

	st := &app.model.PinDialog
	st.Query = "somequery"
	st.Selected = len(items) - 1
	st.Ranked = make([]int, len(items))
	for i := range st.Ranked {
		st.Ranked[i] = i
	}
	termW, termH := screen.Size()
	layout := app.layoutForTerminalSize(termW, termH)
	dialog.EnsurePinListScroll(st, dialog.PinDialogListRows(layout.Height))
	if st.ListScroll == 0 {
		t.Fatal("test setup: expected a non-zero ListScroll to exercise restore")
	}
	wantQuery, wantSelected, wantScroll := st.Query, st.Selected, st.ListScroll

	app.pinCtrl.ViewSelected()

	if app.model.PinDialog.Open {
		t.Fatal("Pin dialog should be hidden while the fullscreen preview is open")
	}
	if !app.model.FullscreenFilePreview.Open {
		t.Fatal("expected the fullscreen preview to be open")
	}

	app.previewCtrl.CloseFilePreviewFullscreen()

	if !app.model.PinDialog.Open {
		t.Fatal("expected the Pin dialog to reopen once the preview closed")
	}
	if app.model.PinDialog.Query != wantQuery {
		t.Fatalf("Query = %q, want %q", app.model.PinDialog.Query, wantQuery)
	}
	if app.model.PinDialog.Selected != wantSelected {
		t.Fatalf("Selected = %d, want %d", app.model.PinDialog.Selected, wantSelected)
	}
	if app.model.PinDialog.ListScroll != wantScroll {
		t.Fatalf("ListScroll = %d, want %d", app.model.PinDialog.ListScroll, wantScroll)
	}
}

// TestPinDialogViewSelectedRestoresImmediatelyOnPreviewOpenError covers the case where the
// fullscreen preview itself fails to open (e.g. a too-small terminal): there is no preview
// session to close later, so the Pin dialog must be restored right away instead of waiting
// for a close hook that will never fire.
func TestPinDialogViewSelectedRestoresImmediatelyOnPreviewOpenError(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Below internal/ui/geom's minWidth/minHeight floor so OpenFullscreenFilePreviewAt's
	// layout check reports TooSmall and returns an error.
	screen := newScreen(t, 10, 5)
	app := newApp(t, screen, dir)

	app.model.PinnedItems = []ui.PinnedItem{{Path: target}}
	app.pinCtrl.OpenDialog()
	app.model.PinDialog.Selected = 0

	app.pinCtrl.ViewSelected()

	if !app.model.PinDialog.Open {
		t.Fatal("expected the Pin dialog to be restored immediately after a failed preview open")
	}
	if app.model.FullscreenFilePreview.Open {
		t.Fatal("expected the fullscreen preview to not be open")
	}
}

// TestFilePreviewFullscreenClosedNoopWhenNotLaunchedFromPin covers the ordinary F3 path (not
// via Pin): closing the fullscreen preview must not touch a Pin dialog it was never asked to
// restore.
func TestFilePreviewFullscreenClosedNoopWhenNotLaunchedFromPin(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if err := app.previewCtrl.OpenFullscreenFilePreviewAt(target); err != nil {
		t.Fatalf("OpenFullscreenFilePreviewAt: %v", err)
	}

	app.previewCtrl.CloseFilePreviewFullscreen()

	if app.model.PinDialog.Open {
		t.Fatal("Pin dialog should not be reopened by a preview close it wasn't asked to restore")
	}
}
