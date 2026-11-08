package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// selectedRows lists which of the first n visible entries are selected.
func selectedRows(p *panel.State, n int) map[int]bool {
	rows := map[int]bool{}
	for i := 0; i < n; i++ {
		if e, _, ok := p.VisibleEntry(i); ok && p.IsSelected(e) {
			rows[i] = true
		}
	}
	return rows
}

func wantRows(t *testing.T, p *panel.State, want ...int) {
	t.Helper()
	got := selectedRows(p, p.VisibleEntryCount())
	if len(got) != len(want) {
		t.Fatalf("selected rows = %v want %v", got, want)
	}
	for _, i := range want {
		if !got[i] {
			t.Fatalf("selected rows = %v want %v", got, want)
		}
	}
}

func TestMouseDragSelectsRange(t *testing.T) {
	for _, tc := range []struct {
		name         string
		from, to, lo int
	}{{"down", 2, 4, 2}, {"up", 4, 2, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := mouseTestApp(t)
			p := &app.model.Primary
			x, y := cellOf(t, app, ui.PrimaryPanel, tc.from)
			app.handleMouse(mouseEv(x, y, tcell.Button1))
			if p.RangeAnchor != "" {
				t.Fatal("a press alone must not set the anchor")
			}
			x, y = cellOf(t, app, ui.PrimaryPanel, 3)
			app.handleMouse(mouseEv(x, y, tcell.Button1))
			if p.RangeAnchor == "" || p.Cursor != 3 {
				t.Fatalf("mid-drag anchor=%q cursor=%d", p.RangeAnchor, p.Cursor)
			}
			if span := p.PendingRange(); !span.Active || span.Lo != min(tc.from, 3) || span.Hi != max(tc.from, 3) {
				t.Fatalf("mid-drag span = %+v", span)
			}
			wantRows(t, p)
			x, y = cellOf(t, app, ui.PrimaryPanel, tc.to)
			app.handleMouse(mouseEv(x, y, tcell.Button1))
			app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
			wantRows(t, p, tc.lo, tc.lo+1, tc.lo+2)
			if p.RangeAnchor != "" || p.Cursor != tc.to {
				t.Fatalf("after release anchor=%q cursor=%d", p.RangeAnchor, p.Cursor)
			}
		})
	}
}

func TestMouseDragAddsToExistingSelection(t *testing.T) {
	app, _ := mouseTestApp(t)
	p := &app.model.Primary
	x, y := cellOf(t, app, ui.PrimaryPanel, 3)
	click(app, x, y, tcell.Button2) // selects 3
	x, y = cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	x, y = cellOf(t, app, ui.PrimaryPanel, 4)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	wantRows(t, p, 2, 3, 4)
}

func TestMouseClickDoesNotSelect(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 3)
	click(app, x, y, tcell.Button1)
	wantRows(t, &app.model.Primary)
	if app.model.Primary.Cursor != 3 || app.model.Primary.RangeAnchor != "" || app.mouse.drag.armed {
		t.Fatalf("cursor=%d anchor=%q drag=%+v", app.model.Primary.Cursor, app.model.Primary.RangeAnchor, app.mouse.drag)
	}
}

func TestMouseDoubleClickDoesNotArmDrag(t *testing.T) {
	app, _ := mouseTestApp(t)
	idx := indexOfEntry(t, &app.model.Primary, "aaa-kettle")
	x, y := cellOf(t, app, ui.PrimaryPanel, idx)
	click(app, x, y, tcell.Button1)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	if app.mouse.drag.armed {
		t.Fatal("double click armed a drag")
	}
}

// A click can scroll the list, so the same screen row then holds another entry; the release must
// still count as a click, in every scroll mode.
func TestMouseClickNearEdgeDoesNotSelect(t *testing.T) {
	for _, mode := range []panel.ScrollMode{panel.ScrollModeCenter, panel.ScrollModeEdge} {
		app, _ := mouseTestApp(t)
		p := &app.model.Primary
		p.ScrollMode = mode
		x, y := cellOf(t, app, ui.PrimaryPanel, app.activeViewportRows()-1)
		click(app, x, y, tcell.Button1)
		wantRows(t, p)
		if p.RangeAnchor != "" || app.mouse.drag.armed {
			t.Fatalf("mode %v: anchor=%q drag=%+v", mode, p.RangeAnchor, app.mouse.drag)
		}
	}
}

// The pointer moving up must move the cursor up even when every move recenters the list.
func TestMouseDragFollowsPointerWhenListScrolls(t *testing.T) {
	app, _ := mouseTestApp(t)
	p := &app.model.Primary
	p.ScrollMode = panel.ScrollModeCenter
	x, y := cellOf(t, app, ui.PrimaryPanel, 3)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	start := p.Cursor
	app.handleMouse(mouseEv(x, y+1, tcell.Button1))
	app.handleMouse(mouseEv(x, y+2, tcell.Button1))
	if p.Cursor != start+2 {
		t.Fatalf("cursor = %d want %d", p.Cursor, start+2)
	}
	app.handleMouse(mouseEv(x, y+1, tcell.Button1))
	if p.Cursor != start+1 {
		t.Fatalf("cursor = %d want %d", p.Cursor, start+1)
	}
	app.handleMouse(mouseEv(x+5, y+1, tcell.Button1)) // sideways: same row, same cursor
	if p.Cursor != start+1 {
		t.Fatalf("sideways motion moved the cursor to %d", p.Cursor)
	}
	app.handleMouse(mouseEv(x, y+1, tcell.ButtonNone))
	if n := p.SelectedPathCount(); n != 2 {
		t.Fatalf("selected = %d want 2", n)
	}
}

func TestMouseDragKeyCancels(t *testing.T) {
	app, _ := mouseTestApp(t)
	p := &app.model.Primary
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	x, y = cellOf(t, app, ui.PrimaryPanel, 4)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if p.RangeAnchor != "" || app.mouse.drag.armed {
		t.Fatalf("anchor=%q drag=%+v", p.RangeAnchor, app.mouse.drag)
	}
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	wantRows(t, p)
}

func TestMouseDragScrollsPastListEdge(t *testing.T) {
	app, _ := mouseTestApp(t)
	p := &app.model.Primary
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	_, h := app.screen.Size()
	below := h - 1 // the footer row, past the list's bottom
	rows := app.activeViewportRows()
	for i := 0; i < rows+2; i++ {
		app.handleMouse(mouseEv(x, below, tcell.Button1))
	}
	if p.Cursor <= rows-1 || p.ScrollOffset == 0 {
		t.Fatalf("cursor=%d scroll=%d rows=%d: did not scroll", p.Cursor, p.ScrollOffset, rows)
	}
	cursor := p.Cursor
	app.handleMouse(mouseEv(x, below, tcell.ButtonNone)) // a release past the edge adds no row
	if p.Cursor != cursor {
		t.Fatalf("release moved the cursor from %d to %d", cursor, p.Cursor)
	}
	if n, want := p.SelectedPathCount(), cursor-2+1; n != want {
		t.Fatalf("selected = %d want %d", n, want)
	}
}

func TestMouseDragStaysWithItsPanel(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	x, y = cellOf(t, app, ui.SecondaryPanel, 4)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if app.model.ActivePanel != ui.PrimaryPanel {
		t.Fatal("drag changed the active panel")
	}
	wantRows(t, &app.model.Primary, 2, 3, 4)
	wantRows(t, &app.model.Secondary)
}

func TestMouseDragOverFooterDoesNotPressIt(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	_, h := app.screen.Size()
	app.handleMouse(mouseEv(3, h-1, tcell.Button1))
	app.handleMouse(mouseEv(3, h-1, tcell.ButtonNone))
	if app.inputMode() != InputModeNormal {
		t.Fatalf("mode = %v: the footer took the drag", app.inputMode())
	}
}

func TestMouseDragWheelExtendsRange(t *testing.T) {
	app, _ := mouseTestApp(t)
	p := &app.model.Primary
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	app.handleMouse(mouseEv(x, y, tcell.WheelDown)) // tcell reports no button bits on a wheel
	if !app.mouse.drag.active || p.Cursor <= 2 {
		t.Fatalf("drag=%+v cursor=%d", app.mouse.drag, p.Cursor)
	}
	x, y = cellOf(t, app, ui.PrimaryPanel, p.Cursor)
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if n, want := p.SelectedPathCount(), p.Cursor-2+1; n != want {
		t.Fatalf("selected = %d want %d", n, want)
	}
}

func TestMouseDragCancelledWhenDialogOpens(t *testing.T) {
	app, _ := mouseTestApp(t)
	p := &app.model.Primary
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	x, y = cellOf(t, app, ui.PrimaryPanel, 4)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	app.finishResolvedKeyboardAction(keymap.ActionPanelSortDialog)
	if app.inputMode() == InputModeNormal {
		t.Fatal("sort dialog did not open")
	}
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if p.RangeAnchor != "" || app.mouse.drag.armed {
		t.Fatalf("anchor=%q drag=%+v", p.RangeAnchor, app.mouse.drag)
	}
	wantRows(t, p)
}

func TestMouseDragHidesHovers(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	x, y = cellOf(t, app, ui.PrimaryPanel, 3)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	if app.mouse.hovers() || app.model.PanelRowHover.Active {
		t.Fatal("hovers must be off while a drag is active")
	}
}

func indexOfEntry(t *testing.T, p *panel.State, name string) int {
	t.Helper()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		if e, _, ok := p.VisibleEntry(i); ok && e.Name == name {
			return i
		}
	}
	t.Fatalf("no entry %q", name)
	return -1
}
