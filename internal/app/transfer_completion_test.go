package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestTransferDialogPrefillDestinationTrailingSlash(t *testing.T) {
	root := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	app := newTestApp(t, screen, testOptions(root))
	if err := app.inactivePanel().Load(root); err != nil {
		t.Fatal(err)
	}

	app.dialogCtrl.OpenCopyDialog()
	dest := app.model.TransferDialog.Destination.Value
	if dest == "" || dest[len(dest)-1] != '/' {
		t.Fatalf("destination prefill = %q, want trailing %q", dest, filepath.Separator)
	}
}

func TestTransferDialogCopyPreserveAltAndFocusedShortcuts(t *testing.T) {
	root := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	cfg.Operations.PreservePermissions = true
	cfg.Operations.PreserveTimestamps = true
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})

	app.dialogCtrl.OpenCopyDialog()
	d := &app.model.TransferDialog
	if !d.Open || d.Kind != dialog.TransferKindCopy {
		t.Fatal("copy dialog should be open")
	}
	if !d.PreservePermissions || !d.PreserveTimestamps {
		t.Fatal("expected preserve options from config defaults")
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if d.PreservePermissions {
		t.Fatal("Alt+R should toggle preserve permissions off")
	}
	if !d.PreserveTimestamps {
		t.Fatal("Alt+T should not affect timestamps yet")
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModAlt))
	if d.PreserveTimestamps {
		t.Fatal("Alt+T should toggle preserve timestamps off")
	}

	d.FocusField = 1
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if !d.PreservePermissions {
		t.Fatal("r on focused permissions row should toggle on")
	}

	d.FocusField = 2
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if !d.PreserveTimestamps {
		t.Fatal("t on focused timestamps row should toggle on")
	}
}

func TestTransferDialogTabAcceptsFilesystemCompletion(t *testing.T) {
	root := t.TempDir()
	fooDir := filepath.Join(root, "foo")
	if err := os.MkdirAll(fooDir, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})

	for _, open := range []func(){app.dialogCtrl.OpenCopyDialog, app.dialogCtrl.OpenMoveDialog} {
		open()
		d := &app.model.TransferDialog
		if !d.Open {
			t.Fatal("transfer dialog should be open")
		}
		prefix := filepath.Join(root, "f")
		d.Destination = dialog.FileDialogField{
			Value:  prefix,
			Cursor: len([]rune(prefix)),
		}
		d.FocusField = 0
		app.dialogCtrl.SyncPathFieldCompletion(&d.Destination, app.dialogCtrl.TransferDestinationTextWidth())
		if len(d.Destination.Completion.Items) != 1 || d.Destination.Completion.Items[0].Name != "foo" {
			t.Fatalf("Completion.Items = %+v, want single foo candidate", d.Destination.Completion.Items)
		}

		app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		want := filepath.Join(root, "foo") + "/"
		if d.Destination.Value != want {
			t.Fatalf("Value = %q want %q", d.Destination.Value, want)
		}
		if d.Destination.Cursor != len([]rune(want)) {
			t.Fatalf("Cursor = %d want %d", d.Destination.Cursor, len([]rune(want)))
		}
		app.dialogCtrl.CloseTransferDialog()
	}
}

// TestTransferDialogSinglePrefixCompletionGhostTabAccepts types a partial into the transfer
// destination that has exactly one prefix match: the dropdown must stay closed (ghost text after
// the caret instead of a one-row list), and Tab must accept the ghost suggestion.
func TestTransferDialogSinglePrefixCompletionGhostTabAccepts(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "lantern-orchard")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A single child would otherwise be a ghost candidate for the empty segment after Tab.
	if err := os.WriteFile(filepath.Join(targetDir, "pebble-notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	app := newTestApp(t, screen, testOptions(root))
	app.dialogCtrl.OpenCopyDialog()
	d := &app.model.TransferDialog
	if !d.Open {
		t.Fatal("transfer dialog should be open")
	}
	d.FocusField = 0

	partial := filepath.Join(root, "lantern-o")
	d.Destination = dialog.FileDialogField{Value: partial, Cursor: len([]rune(partial))}
	app.dialogCtrl.SyncPathFieldCompletion(&d.Destination, app.dialogCtrl.TransferDestinationTextWidth())

	if len(d.Destination.Completion.Items) != 1 || d.Destination.Completion.Items[0].Name != "lantern-orchard" {
		t.Fatalf("Completion.Items = %+v, want single lantern-orchard candidate", d.Destination.Completion.Items)
	}
	if d.Destination.Completion.Open {
		t.Fatal("a single prefix candidate must not open the dropdown")
	}
	if got := d.Destination.Completion.GhostSuffix(d.Destination.Value); got != "rchard" {
		t.Fatalf("GhostSuffix = %q, want %q", got, "rchard")
	}

	// Tab accepts the ghost suggestion (appending "/" for a directory).
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	want := targetDir + "/"
	if d.Destination.Value != want {
		t.Fatalf("Value after Tab = %q, want %q", d.Destination.Value, want)
	}
	// Nothing is suggested for the empty segment until a letter or another Tab.
	c := &d.Destination.Completion
	if c.Open || c.GhostSuffix(d.Destination.Value) != "" {
		t.Fatalf("after Tab-accept: Open=%v ghost=%q, want neither", c.Open, c.GhostSuffix(d.Destination.Value))
	}
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if d.Destination.Value != want+"pebble-notes.txt" {
		t.Fatalf("second Tab: Value = %q, want the sole child accepted", d.Destination.Value)
	}
}

// TestTransferDialogEnterOverGhostTextConfirmsNotAccepts confirms that Enter, pressed while only
// ghost text (a single prefix candidate) is showing, is not swallowed by the completion handler:
// it falls through to the dialog's own Enter handling (which confirms the transfer and starts a
// job) instead of inserting the ghost suggestion into the destination field.
func TestTransferDialogEnterOverGhostTextConfirmsNotAccepts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "meadow.txt"))
	targetDir := filepath.Join(dir, "lantern-orchard")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.dialogCtrl.OpenCopyDialog()
	d := &app.model.TransferDialog
	if !d.Open {
		t.Fatal("transfer dialog should be open")
	}
	d.FocusField = 0

	partial := filepath.Join(dir, "lantern-o")
	d.Destination = dialog.FileDialogField{Value: partial, Cursor: len([]rune(partial))}
	app.dialogCtrl.SyncPathFieldCompletion(&d.Destination, app.dialogCtrl.TransferDestinationTextWidth())
	if d.Destination.Completion.Open || d.Destination.Completion.GhostSuffix(d.Destination.Value) == "" {
		t.Fatalf("expected ghost text, got Completion=%+v", d.Destination.Completion)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("Enter over ghost text should confirm and close the dialog, not accept the ghost")
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job after Enter, got %d", len(app.jobState.AllJobs()))
	}
}

// TestTransferDialogFuzzyCompletionDropdownDownEnter types a fuzzy (non-prefix) partial into
// the transfer destination, checks the dropdown opens with both fuzzy matches, uses Down to
// select the second row, accepts it with Enter, and checks a second Enter is no longer consumed
// by the (now closed) dropdown.
func TestTransferDialogFuzzyCompletionDropdownDownEnter(t *testing.T) {
	// The active panel's own directory is kept empty and separate from targetRoot (where the
	// completion candidates live) so confirming the transfer with no selection can't kick off a
	// real copy job — it only has to prove the second Enter reached the dialog's own Enter
	// handling instead of being re-swallowed by the (by-then-closed) completion dropdown.
	activeRoot := t.TempDir()
	targetRoot := t.TempDir()
	// Neither name is a prefix of "gully"; both fuzzy-match it as a contiguous substring.
	first := filepath.Join(targetRoot, "barnacle-gully-meadow")
	second := filepath.Join(targetRoot, "copper-gully-thicket")
	for _, p := range []string{first, second} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	app := newTestApp(t, screen, testOptions(activeRoot))
	app.dialogCtrl.OpenCopyDialog()
	d := &app.model.TransferDialog
	if !d.Open {
		t.Fatal("transfer dialog should be open")
	}
	d.FocusField = 0

	partial := filepath.Join(targetRoot, "gully")
	d.Destination = dialog.FileDialogField{Value: partial, Cursor: len([]rune(partial))}
	app.dialogCtrl.SyncPathFieldCompletion(&d.Destination, app.dialogCtrl.TransferDestinationTextWidth())

	if !d.Destination.Completion.Open || len(d.Destination.Completion.Items) != 2 {
		t.Fatalf("Completion = %+v, want dropdown open with 2 fuzzy candidates", d.Destination.Completion)
	}
	if d.Destination.Completion.Selected != 0 {
		t.Fatalf("Selected = %d, want 0 before Down", d.Destination.Completion.Selected)
	}
	secondName := d.Destination.Completion.Items[1].Name

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if d.Destination.Completion.Selected != 1 {
		t.Fatalf("Selected after Down = %d, want 1", d.Destination.Completion.Selected)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	wantValue := filepath.Join(targetRoot, secondName) + "/"
	if d.Destination.Value != wantValue {
		t.Fatalf("Value after Enter = %q want %q", d.Destination.Value, wantValue)
	}
	if d.Destination.Completion.Open {
		t.Fatal("dropdown should close after accepting")
	}
	if !d.Open {
		t.Fatal("transfer dialog should still be open after accepting the completion")
	}

	// The dropdown is closed and the newly accepted (empty) directory has nothing to complete,
	// so a second Enter must not be swallowed by the completion handler; it falls through to
	// the dialog's normal Enter handling instead of re-accepting or reopening the dropdown.
	valueBefore := d.Destination.Value
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if d.Destination.Value != valueBefore {
		t.Fatalf("second Enter changed Value to %q, want unchanged %q", d.Destination.Value, valueBefore)
	}
	if d.Destination.Completion.Open {
		t.Fatal("second Enter should not reopen the dropdown")
	}
}

// TestTransferDialogCompletionDirsOnlyForMultiSelection confirms that copying several entries
// completes only directories, while copying a single file still offers files.
func TestTransferDialogCompletionDirsOnlyForMultiSelection(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, "harbor-lantern")
	filePath := filepath.Join(root, "harbor-ledger.txt")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)
	app := newTestApp(t, screen, testOptions(root))

	complete := func(selected map[string]bool) []string {
		app.activePanel().SelectedPaths = selected
		app.dialogCtrl.OpenCopyDialog()
		d := &app.model.TransferDialog
		partial := filepath.Join(root, "harbor-")
		d.Destination.Value, d.Destination.Cursor, d.Destination.PrefillPending = partial, len([]rune(partial)), false
		app.dialogCtrl.SyncPathFieldCompletion(&d.Destination, app.dialogCtrl.TransferDestinationTextWidth())
		var names []string
		for _, it := range d.Destination.Completion.Items {
			names = append(names, it.Name)
		}
		return names
	}

	if got := complete(map[string]bool{dirPath: true, filePath: true}); len(got) != 1 || got[0] != "harbor-lantern" {
		t.Fatalf("multi-selection completion = %v, want [harbor-lantern]", got)
	}
	if got := complete(map[string]bool{filePath: true}); len(got) != 2 {
		t.Fatalf("single-file completion = %v, want both entries", got)
	}
}

// TestTransferDialogTypedSinglePrefixPaintsGhost confirms that typing into the Destination
// field paints the single prefix candidate's remainder as ghost text on screen.
func TestTransferDialogTypedSinglePrefixPaintsGhost(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"velvet-compass", "velour-garden", "copper-kettle"} {
		if err := os.Mkdir(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(120, 30)
	app := newTestApp(t, screen, testOptions(root))
	app.dialogCtrl.OpenCopyDialog()
	typed := filepath.Join(root, "velvet-co")
	for _, r := range typed {
		app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.render()

	cells, w, h := screen.GetContents()
	want := "/velvet-compass" // the long temp path may scroll; panels list the name without "/"
	for y := range h {
		var sb strings.Builder
		for x := range w {
			sb.WriteString(string(cells[y*w+x].Runes))
		}
		if strings.Contains(sb.String(), want) {
			return
		}
	}
	t.Fatalf("no screen row shows %q (typed %q plus ghost suffix)", want, typed)
}
