package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for Jump to directory over the find fixture tree (findKeyFixture). Every key
// is followed by settleJump, which drains the walk and the match passes so the transcript never
// depends on timing. Paths are reported relative to the fixture root.

// settleJump runs the dialog's posted work (scan events, due matches) until the walk and the
// current query's match are done.
func settleJump(t *testing.T, app *App) {
	t.Helper()
	screen := app.screen.(tcell.SimulationScreen)
	deadline := time.Now().Add(raceSlowdown * 3 * time.Second)
	for time.Now().Before(deadline) {
		for screen.HasPendingEvent() {
			if ev, ok := screen.PollEvent().(*tcell.EventInterrupt); ok {
				app.handleInterruptPayload(ev.Data())
				app.reconcileAfterEvent()
			}
		}
		st := &app.model.JumpDialog
		if !st.Open || (!st.Scanning && !st.Searching) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("jump dialog did not settle in time")
}

// jumpKeyFocus is the focused widget's position in the dialog's focus order (0 = the query,
// which also drives the list).
func jumpKeyFocus(st *dialog.JumpDialogState) int {
	ids := []dlg.ID{dialog.JumpQuery, dialog.JumpDepth, dialog.JumpHidden, dialog.JumpStayOnVol, dlg.OK, dlg.Cancel}
	if st.Focus == "" {
		return 0
	}
	for i, id := range ids {
		if id == st.Focus {
			return i
		}
	}
	return -1
}

func jumpKeyState(app *App) string {
	st := app.model.JumpDialog
	cursor := ""
	if r, ok := st.Cursor(); ok {
		cursor = r.Rel
	}
	return fmt.Sprintf("focus=%d query=%q qcursor=%d depth=%q sel=%d cursor=%q scroll=%d results=%d matched=%d hidden=%v volume=%v",
		jumpKeyFocus(&st), st.Query, st.QueryCursor, st.DepthInput, st.Selected, cursor, st.ListScroll, len(st.Results), st.Matched,
		st.IncludeHidden, st.StayOnVolume)
}

func jumpKeyDialog() keyGoldenDialog {
	return keyGoldenDialog{
		name: "jump_dialog",
		setup: func(t *testing.T, dir string) *App {
			findKeyFixture(t, dir)
			app := newApp(t, newScreen(t, 80, 24), dir)
			findKeyRoots.Store(app, dir)
			t.Cleanup(func() { findKeyRoots.Delete(app) })
			app.model.Message = ""
			app.openJumpDialog(ui.PrimaryPanel)
			settleJump(t, app)
			return app
		},
		settle:  settleJump,
		state:   jumpKeyState,
		open:    func(app *App) bool { return app.model.JumpDialog.Open },
		outcome: findKeyOutcome,
	}
}

func TestJumpDialogKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, jumpKeyDialog(), []simpleKeyScript{
		{"walk-down", "Down Down Down Down Down Down Down Down Down"},
		{"query-then-nav", findQuery("ju") + " Down Up"},
		{"query-no-match", findQuery("zzq") + " Enter"},
		{"mnemonics", "Alt+i Down Alt+v Alt+i"},
		{"depth-typing", "Tab 1 Tab 0"},
		{"query-enter", findQuery("quartz") + " Enter"},
		{"enter-first", "Enter"},
		{"esc", findQuery("har") + " Esc"},
		{"opener-closes", "Down Alt+c"},
		{"open-in-other-panel", "Down Shift+Right"},
		{"tab-to-ok", "Down Tab Tab Tab Tab Enter"},
	})
}
