package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// typeRunes feeds each rune of s through handleKey as a plain (unmodified) rune key.
func typeRunes(t *testing.T, app *App, s string) {
	t.Helper()
	for _, r := range s {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatalf("handleKey(%q) quit", r)
		}
	}
}

func TestFilterDialogMAltFOpensAndNarrows(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "walnut.txt"))
	writeFile(t, filepath.Join(dir, "sesame.txt"))
	if err := os.Mkdir(filepath.Join(dir, "walnutdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if got := app.activePanel().VisibleEntryCount(); got != 3 {
		t.Fatalf("initial VisibleEntryCount = %d, want 3", got)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt)); quit {
		t.Fatal("M-f handleKey quit = true")
	}
	if !app.model.FilterDialog.Open {
		t.Fatal("expected Filter dialog open after M-f")
	}
	if app.model.FilterDialog.Focus != dialog.FilterFocusPattern {
		t.Fatalf("Focus = %d, want FilterFocusPattern", app.model.FilterDialog.Focus)
	}

	typeRunes(t, app, "walnut*")
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter handleKey quit = true")
	}
	if app.model.FilterDialog.Open {
		t.Fatal("expected Filter dialog closed after confirm")
	}

	p := app.activePanel()
	if got := p.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after filter = %d, want 2", got)
	}
	if p.ActiveEntryFilter == nil || p.ActiveEntryFilter.Label != "Filter: walnut*" {
		t.Fatalf("ActiveEntryFilter = %+v, want label %q", p.ActiveEntryFilter, "Filter: walnut*")
	}

	// Empty pattern + OK clears the active filter.
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt)); quit {
		t.Fatal("M-f handleKey quit = true")
	}
	if !app.model.FilterDialog.Open {
		t.Fatal("expected Filter dialog open")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter handleKey quit = true")
	}
	if app.model.FilterDialog.Open {
		t.Fatal("expected Filter dialog closed after empty-pattern OK")
	}
	if app.activePanel().ActiveEntryFilter != nil {
		t.Fatalf("expected filter cleared, got %+v", app.activePanel().ActiveEntryFilter)
	}
	if got := app.activePanel().VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount after clear = %d, want 3", got)
	}
}

func TestFilterDialogOnlyMetaColumnsMatchesMetaValue(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "walnut.txt"))
	writeFile(t, filepath.Join(dir, "sesame.txt"))
	writeFile(t, filepath.Join(dir, "gravel.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// A meta column is active on the panel, with a value only gravel.txt carries — its basename
	// doesn't contain the pattern used below, so a match proves the meta value was consulted.
	app.model.MetaResults[app.model.ActivePanel] = []ui.MetaColumnState{{
		EntryName:   "tag",
		ColumnTitle: "Tag",
		Results:     map[string]string{filepath.Join(dir, "gravel.txt"): "harbor light"},
	}}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt)); quit {
		t.Fatal("M-f handleKey quit = true")
	}
	if !app.model.FilterDialog.Open {
		t.Fatal("expected Filter dialog open after M-f")
	}
	if app.model.FilterDialog.MetaColumnCount != 1 {
		t.Fatalf("MetaColumnCount = %d, want 1", app.model.FilterDialog.MetaColumnCount)
	}
	if !app.model.FilterDialog.IncludeMetaColumns {
		t.Fatal("expected IncludeMetaColumns checked by default when meta columns are active")
	}

	// Type the pattern (shell glob, the dialog's default mode) while focus is still on the
	// Pattern field, then Alt+N (Only meta columns) — which moves focus onto the checkbox row,
	// so it must come after typing.
	typeRunes(t, app, "*harbor*")
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModAlt)); quit {
		t.Fatal("M-N handleKey quit = true")
	}
	if !app.model.FilterDialog.OnlyMetaColumns || !app.model.FilterDialog.IncludeMetaColumns {
		t.Fatalf("expected Only+Include meta columns set, got %+v", app.model.FilterDialog)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter handleKey quit = true")
	}
	if app.model.FilterDialog.Open {
		t.Fatal("expected Filter dialog closed after confirm")
	}

	p := app.activePanel()
	if got := p.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount after meta filter = %d, want 1", got)
	}
	entry, _, ok := p.VisibleEntry(0)
	if !ok || entry.Name != "gravel.txt" {
		t.Fatalf("VisibleEntry(0) = %+v, ok=%v, want gravel.txt", entry, ok)
	}
	if p.ActiveEntryFilter == nil || p.ActiveEntryFilter.ID != panel.PatternMetaFilterID {
		t.Fatalf("ActiveEntryFilter.ID = %+v, want %q", p.ActiveEntryFilter, panel.PatternMetaFilterID)
	}
}
