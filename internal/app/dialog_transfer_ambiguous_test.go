package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
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

	// Enter on OK (default focus) navigates to the common root.
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should close on OK")
	}
	if got := app.activePanel().PathString(); got != root {
		t.Fatalf("panel path = %q, want common root %q", got, root)
	}

	// From the common root the transfer dialog opens directly.
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog must not open at the common root")
	}
	if !app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should open at the common root")
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

	app.activateMoveAction()
	app.handleAmbiguousTransferKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt))
	if app.model.AmbiguousTransfer.Open {
		t.Fatal("ambiguous dialog should close on Alt+C")
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
