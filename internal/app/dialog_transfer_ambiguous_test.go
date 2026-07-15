package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func setupAmbiguousSelection(t *testing.T) (app *App, root, alpha, bravo string) {
	t.Helper()
	root = t.TempDir()
	alpha = filepath.Join(root, "alpha")
	bravo = filepath.Join(root, "bravo")
	for _, d := range []string{alpha, bravo} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	writeFile(t, filepath.Join(alpha, "river.txt"))
	writeFile(t, filepath.Join(alpha, "pond.txt"))
	writeFile(t, filepath.Join(bravo, "stone.txt"))
	screen := newScreen(t, 80, 24)
	app = newApp(t, screen, alpha)
	return app, root, alpha, bravo
}

func TestTransferMultiDirSelectionOpensAmbiguousDialog(t *testing.T) {
	app, root, alpha, bravo := setupAmbiguousSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should open for multi-directory selection away from common root")
	}
	if app.model.TransferDialog.Open {
		t.Fatal("transfer dialog must not open when ambiguous dialog triggers")
	}
	if app.model.AmbiguousTransfer.CommonRoot != root {
		t.Fatalf("common root = %q, want %q", app.model.AmbiguousTransfer.CommonRoot, root)
	}
	if got := app.model.AmbiguousTransfer.Kind; got != dialog.TransferKindCopy {
		t.Fatalf("kind = %v, want Copy", got)
	}
	wantLabels := map[string]bool{"alpha/river.txt": true, "bravo/stone.txt": true}
	if len(app.model.AmbiguousTransfer.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(app.model.AmbiguousTransfer.Entries))
	}
	for _, e := range app.model.AmbiguousTransfer.Entries {
		if !wantLabels[e.Name] {
			t.Errorf("unexpected entry label %q", e.Name)
		}
	}

	// Enter on OK (default focus) opens the transfer dialog directly; no navigation.
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should close on OK")
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want unchanged %q", got, alpha)
	}
	if !app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should open on OK")
	}
	if app.model.TransferDialog.Kind != dialog.TransferKindCopy {
		t.Fatalf("transfer dialog kind = %v, want Copy", app.model.TransferDialog.Kind)
	}
}

func TestTransferMultiDirSelectionMoveKind(t *testing.T) {
	app, _, alpha, bravo := setupAmbiguousSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	app.activateMoveAction()
	if !app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should open for move")
	}
	if got := app.model.AmbiguousTransfer.Kind; got != dialog.TransferKindMove {
		t.Fatalf("kind = %v, want Move", got)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindMove {
		t.Fatal("transfer dialog should open with Move kind")
	}
}

func TestTransferAmbiguousDialogCancelKeepsLocation(t *testing.T) {
	app, _, alpha, bravo := setupAmbiguousSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	app.activateMoveAction()
	if !app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should open for move too")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should close on Esc")
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want unchanged %q", got, alpha)
	}
	if app.model.DestinationTargetPrimary || app.model.DestinationTargetSecondary {
		t.Fatal("cancel should clear destination target panels")
	}

	app.activateMoveAction()
	app.handleAmbiguousTransferKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt))
	if app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should close on Alt+C")
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want unchanged %q", got, alpha)
	}
	if app.model.DestinationTargetPrimary || app.model.DestinationTargetSecondary {
		t.Fatal("Alt+C should clear destination target panels")
	}
}

func TestTransferAmbiguousDialogMarksInactiveDestinationTarget(t *testing.T) {
	app, _, alpha, bravo := setupAmbiguousSelection(t)
	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	app.activateCopyAction()
	if !app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should open")
	}
	if !app.model.DestinationTargetSecondary {
		t.Fatal("expected Secondary (inactive) panel marked as destination target")
	}
	if app.model.DestinationTargetPrimary {
		t.Fatal("Primary panel should not be marked as destination target")
	}
}

func TestOpenSelectionsRootNavigatesToCommonRoot(t *testing.T) {
	app, root, alpha, bravo := setupAmbiguousSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	ev := tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModAlt)
	if quit, _ := app.handleKey(ev); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != root {
		t.Fatalf("panel path = %q, want common root %q", got, root)
	}

	// Already at the root: stays put.
	if quit, _ := app.handleKey(ev); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != root {
		t.Fatalf("panel path = %q, want unchanged %q", got, root)
	}
}

func TestOpenSelectionsRootSingleDirNavigatesToParent(t *testing.T) {
	app, _, alpha, bravo := setupAmbiguousSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(alpha, "pond.txt"))
	if err := p.Load(bravo); err != nil {
		t.Fatalf("load bravo: %v", err)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModAlt)); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want single selection dir %q", got, alpha)
	}
}

func TestOpenSelectionsRootWithoutSelectionsKeepsLocation(t *testing.T) {
	app, _, alpha, _ := setupAmbiguousSelection(t)

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModAlt)); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want unchanged %q", got, alpha)
	}
}

func TestTransferSingleDirSelectionElsewhereSkipsAmbiguousDialog(t *testing.T) {
	app, _, alpha, bravo := setupAmbiguousSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(alpha, "pond.txt"))
	if err := p.Load(bravo); err != nil {
		t.Fatalf("load bravo: %v", err)
	}

	app.activateCopyAction()
	if app.model.AmbiguousTransfer.Open {
		t.Fatal("single-directory selection must not trigger ambiguous dialog")
	}
	if !app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should open for single-directory selection")
	}
}
