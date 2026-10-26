package app

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Invariants every dlg dialog keeps, checked on the opened dialog of each key-script fixture (once
// per fixture, from runKeyGolden), so a new dialog is covered as soon as it has key scripts. They
// tie the spec to what the user gets: the mouse table and the drawn screen agree with the spec,
// every widget can be reached and named, and the spec's OK / Cancel have a central action.

var dialogInvariantsChecked sync.Map // fixture name -> struct{}

// checkDialogInvariantsOnce runs checkDialogInvariants the first time fixture opens.
func checkDialogInvariantsOnce(t *testing.T, fixture string, app *App) {
	t.Helper()
	if _, seen := dialogInvariantsChecked.LoadOrStore(fixture, struct{}{}); !seen {
		checkDialogInvariants(t, app)
	}
}

func checkDialogInvariants(t *testing.T, app *App) {
	t.Helper()
	app.render()
	dm, ok := app.activeDialogMouse()
	if !ok {
		t.Errorf("invariant: input mode %v has no activeDialogMouse entry", app.inputMode())
		return
	}
	spec := dm.spec
	w, h := app.screen.Size()
	p, placed := dlg.Place(w, h, spec)
	if !placed {
		return // too small to draw: nothing on screen to check
	}

	// Widget IDs are unique, and mnemonics are unique and shown in their labels.
	seenID := map[dlg.ID]bool{}
	seenMnemonic := map[rune]string{}
	focusable := map[dlg.ID]bool{}
	check := func(id dlg.ID, label string, mnemonic rune, canFocus bool) {
		if id != "" {
			if seenID[id] {
				t.Errorf("invariant: widget ID %q used twice", id)
			}
			seenID[id] = true
			if canFocus {
				focusable[id] = true
			}
		}
		if mnemonic == 0 {
			return
		}
		m := unicode.ToLower(mnemonic)
		if prev, dup := seenMnemonic[m]; dup {
			t.Errorf("invariant: mnemonic %q on both %q and %q", m, prev, label)
		}
		seenMnemonic[m] = label
		if !strings.ContainsRune(strings.ToLower(label), m) {
			t.Errorf("invariant: mnemonic %q does not appear in label %q", m, label)
		}
	}
	for _, it := range dlg.Widgets(spec) {
		check(it.ID, it.Label, it.Mnemonic, it.Focusable())
	}
	for _, b := range spec.Buttons {
		check(b.ID, b.Label, b.Mnemonic, true)
	}

	// Every focusable widget is reachable from the initial focus with Tab and the arrow keys.
	keys := []tcell.Key{tcell.KeyTab, tcell.KeyBacktab, tcell.KeyDown, tcell.KeyUp, tcell.KeyLeft, tcell.KeyRight}
	start := dlg.Focused(spec, *dm.focus)
	reached := map[dlg.ID]bool{start: true}
	for queue := []dlg.ID{start}; len(queue) > 0; queue = queue[1:] {
		for _, k := range keys {
			f := queue[0]
			dlg.HandleKey(spec, &f, tcell.NewEventKey(k, 0, tcell.ModNone))
			if !reached[f] {
				reached[f] = true
				queue = append(queue, f)
			}
		}
	}
	for id := range focusable {
		if !reached[id] {
			t.Errorf("invariant: widget %q cannot be reached with Tab or the arrow keys", id)
		}
	}

	// Labels are drawn where the spec places them, and a press there targets the widget.
	screen := app.screen.(tcell.SimulationScreen)
	for _, pi := range p.Items {
		if pi.ID == "" || !focusable[pi.ID] || pi.Rect.Width <= 0 || pi.Rect.Y >= p.ContentBottom && pi.Rect.Y < p.ButtonY {
			continue
		}
		if label := strings.TrimSpace(pi.Label); label != "" && !coveredByCompletion(app, screen, pi.Rect) {
			if got := tcelltest.TextAt(screen, pi.Rect.X, pi.Rect.Y, pi.Rect.Width); !strings.Contains(got, label) {
				t.Errorf("invariant: %q shows %q at its rect, want its label %q", pi.ID, got, label)
			}
		}
		tg, ok := dlg.TargetAt(spec, w, h, pi.Rect.X+pi.Rect.Width/2, pi.Rect.Y)
		if !ok || tg.ID != pi.ID {
			t.Errorf("invariant: TargetAt on %q = (%+v, %v)", pi.ID, tg, ok)
		}
	}

	// The spec's OK / Cancel buttons have a central action.
	b := app.activeDialogButtons(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if seenID[dlg.OK] && b.OK == nil {
		t.Errorf("invariant: the spec has an OK button but activeDialogButtons has no OK")
	}
	if seenID[dlg.Cancel] && b.Cancel == nil {
		t.Errorf("invariant: the spec has a Cancel button but activeDialogButtons has no Cancel")
	}
}

// coveredByCompletion reports whether a path-completion dropdown, painted over the dialog after
// the spec, covers part of r.
func coveredByCompletion(app *App, screen tcell.SimulationScreen, r dlg.Rect) bool {
	var dropdown []tcell.Color
	for _, st := range []tcell.Style{app.styles.DialogCompletionItem, app.styles.DialogCompletionItemSelected} {
		_, bg, _ := st.Decompose()
		dropdown = append(dropdown, bg)
	}
	for x := r.X; x < r.X+r.Width; x++ {
		_, st, _ := screen.Get(x, r.Y)
		if _, bg, _ := st.Decompose(); slices.Contains(dropdown, bg) {
			return true
		}
	}
	return false
}
