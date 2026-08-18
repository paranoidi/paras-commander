package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// TestViMotionToggleEsc verifies Esc toggles vi-motion mode on/off in a plain browser view.
func TestViMotionToggleEsc(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if app.model.ViMotionMode {
		t.Fatal("ViMotionMode should start false")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)); quit {
		t.Fatal("handleKey(Esc) quit = true, want false")
	}
	if !app.model.ViMotionMode {
		t.Fatal("Esc should toggle ViMotionMode on")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ViMotionMode {
		t.Fatal("second Esc should toggle ViMotionMode back off")
	}
}

// TestViMotionHJKLActLikeArrowsOnlyWhenModeOn verifies h/j/k/l move the cursor like the
// arrow-key nav actions while vi-motion mode is on, and are ordinary quick-filter text
// (no cursor movement) while it is off.
func TestViMotionHJKLActLikeArrowsOnlyWhenModeOn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Mode off: 'j' is plain filter text, not cursor-down.
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	if app.model.Primary.Cursor != 0 {
		t.Fatalf("cursor moved with vi-motion off: got %d, want 0", app.model.Primary.Cursor)
	}
	if got := app.model.Primary.Filter.Query; got != "j" {
		t.Fatalf("quick filter query = %q, want %q", got, "j")
	}
	app.activePanel().CancelFilter(app.activeViewportRows())

	// Mode on: hjkl act exactly like the arrow-key nav actions (nav.down/nav.up here) and
	// never fall through to the quick filter.
	app.model.ViMotionMode = true
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	if app.model.Primary.Cursor != 1 {
		t.Fatalf("'j' with vi-motion on: cursor = %d, want 1", app.model.Primary.Cursor)
	}
	if got := app.model.Primary.Filter.Query; got != "" {
		t.Fatalf("'j' with vi-motion on should not touch the quick filter, query = %q", got)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if app.model.Primary.Cursor != 0 {
		t.Fatalf("'k' with vi-motion on: cursor = %d, want 0", app.model.Primary.Cursor)
	}
}

// TestViMotionLeaderLetterDispatchesDirectlyOnlyWhenModeOn verifies a bound leader-menu
// letter (here 'c', file.copy) fires its action directly while vi-motion mode is on, and
// starts the quick filter as usual while it is off.
func TestViMotionLeaderLetterDispatchesDirectlyOnlyWhenModeOn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Mode off: 'c' is plain filter text, copy dialog stays closed.
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("'c' with vi-motion off must not open the copy dialog")
	}
	if got := app.model.Primary.Filter.Query; got != "c" {
		t.Fatalf("quick filter query = %q, want %q", got, "c")
	}
	app.activePanel().CancelFilter(app.activeViewportRows())

	// Mode on: 'c' fires file.copy directly without opening the quick filter first.
	app.model.ViMotionMode = true
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if !app.model.TransferDialog.Open {
		t.Fatal("'c' with vi-motion on should open the copy dialog directly")
	}
	if got := app.model.Primary.Filter.Query; got != "" {
		t.Fatalf("'c' with vi-motion on should not touch the quick filter, query = %q", got)
	}
}

// TestViMotionEscCancelsFilterWithoutTogglingMode verifies Esc while the quick filter is
// active only cancels the filter and does not also flip vi-motion mode as a side effect.
func TestViMotionEscCancelsFilterWithoutTogglingMode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if got := app.model.Primary.Filter.Query; got != "x" {
		t.Fatalf("quick filter query = %q, want %q", got, "x")
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)); quit {
		t.Fatal("handleKey(Esc) quit = true, want false")
	}
	if app.model.Primary.Filter.Editing || app.model.Primary.Filter.Active || app.model.Primary.Filter.Query != "" {
		t.Fatalf("Esc should cancel the quick filter, got editing=%v active=%v query=%q",
			app.model.Primary.Filter.Editing, app.model.Primary.Filter.Active, app.model.Primary.Filter.Query)
	}
	if app.model.ViMotionMode {
		t.Fatal("Esc cancelling the quick filter must not also toggle ViMotionMode")
	}
}

// TestViMotionFooterKeysSwapLettersAndDropShift verifies the footer's F-key entries switch to
// their vi-motion leader letters (when bound) and drop HintShiftPrefix while vi-motion mode is
// on, and are unaffected while it is off.
func TestViMotionFooterKeysSwapLettersAndDropShift(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	findByHint := func(keys []menu.FunctionKey, hint string) menu.FunctionKey {
		t.Helper()
		for _, fk := range keys {
			if fk.Hint == hint {
				return fk
			}
		}
		t.Fatalf("no footer key with Hint %q", hint)
		return menu.FunctionKey{}
	}

	keys := app.activeFooterKeys()
	edit := findByHint(keys, "Edit")
	if edit.KeyLabel != "F4" {
		t.Fatalf("vi-motion off: Edit KeyLabel = %q, want %q", edit.KeyLabel, "F4")
	}
	mov := findByHint(keys, "Mov")
	if mov.HintShiftPrefix == "" {
		t.Fatal("vi-motion off: Mov HintShiftPrefix should be non-empty (Ren)")
	}

	app.model.ViMotionMode = true
	keys = app.activeFooterKeys()
	edit = findByHint(keys, "Edit")
	if edit.KeyLabel != "e" {
		t.Fatalf("vi-motion on: Edit KeyLabel = %q, want %q", edit.KeyLabel, "e")
	}
	menuKey := findByHint(keys, "Menu")
	if menuKey.KeyLabel != "F9" {
		t.Fatalf("vi-motion on: Menu KeyLabel = %q, want %q (no LeaderKey bound)", menuKey.KeyLabel, "F9")
	}
	mov = findByHint(keys, "Mov")
	if mov.HintShiftPrefix != "" {
		t.Fatalf("vi-motion on: Mov HintShiftPrefix = %q, want empty", mov.HintShiftPrefix)
	}
}
