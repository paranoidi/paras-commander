package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the find dialog (and its jump-to-directory mode) over a fixed fixture
// tree. Every key is followed by settleFind, which drains indexing and ranking so the transcript
// never depends on timing. Paths are reported relative to the fixture root.

// findKeyRoots maps each script's App to its fixture root (scripts run in parallel).
var findKeyRoots sync.Map

func findKeyFixture(t *testing.T, dir string) {
	t.Helper()
	files := map[string]int{
		"amber/birch_notes.md":           120,
		"amber/cinder_report.txt":        340,
		"amber/delta/ember_photo.png":    2048,
		"birch/fable_draft.go":           512,
		"birch/glacier_summary.toml":     64,
		"cinder/harbor_map.zip":          4096,
		"cinder/ivory_clip.mp4":          8192,
		"delta/juniper/kestrel_log.txt":  256,
		"delta/juniper/lantern_plan.md":  128,
		"harbor/meadow_index.go":         768,
		"harbor/quartz/thistle_note.txt": 96,
		"willow_overview.md":             300,
		".hidden_orchard/secret.txt":     10,
		".willow_settings":               20,
	}
	for rel, size := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x", size)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// settleFind drains the find dialog's asynchronous index and rank work.
func settleFind(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(raceSlowdown * 3 * time.Second)
	for time.Now().Before(deadline) {
		app.findCtrl.PollUpdates(find.WakePayload{})
		app.findCtrl.HandleThrottleRankWake()
		app.findCtrl.HandleDebounceRankWake()
		app.findCtrl.ApplyPendingRank()
		st := &app.model.FindDialog
		if !st.Open || (st.IndexDone && !st.RankPending) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("find dialog did not settle in time")
}

func findKeyRel(app *App, p string) string {
	root, _ := findKeyRoots.Load(app)
	if rel, err := filepath.Rel(root.(string), p); err == nil {
		return filepath.ToSlash(rel)
	}
	return p
}

// findKeyFocus is the focused widget's position in the dialog's focus order (0 = the query, which
// also drives the list; the selections row only when shown).
func findKeyFocus(st *dialog.FindDialogState) int {
	ids := []dlg.ID{dialog.FindQuery, dialog.FindDepth, dialog.FindOnlyDirs, dialog.FindOnlyFiles, dialog.FindHidden, dialog.FindStayOnVol}
	if st.ShowSearchSelectionsOption {
		ids = append(ids, dialog.FindSelections)
	}
	ids = append(ids, dlg.OK, dlg.Cancel)
	if st.QueryFocused() {
		return 0
	}
	for i, id := range ids {
		if id == st.Focus {
			return i
		}
	}
	return -1
}

func findKeyState(app *App) string {
	st := app.model.FindDialog
	cursor := ""
	if st.Selected >= 0 && st.Selected < len(st.Ranked) {
		if e, ok := st.FindEntryAt(st.Ranked[st.Selected]); ok {
			cursor = e.RelLine
		}
	}
	var marked []string
	for p := range st.MarkedPaths {
		marked = append(marked, findKeyRel(app, p))
	}
	sort.Strings(marked)
	return fmt.Sprintf("focus=%d query=%q qcursor=%d depth=%q sel=%d cursor=%q scroll=%d ranked=%d dirs=%v files=%v hidden=%v volume=%v selections=%v marked=%v",
		findKeyFocus(&st), st.Query, st.QueryCursor, st.DepthInput, st.Selected, cursor, st.ListScroll, len(st.Ranked),
		st.OnlyDirectories, st.OnlyFiles, st.IncludeHidden, st.StayOnCurrentVolume, st.SearchOnlySelections, marked)
}

func findKeyOutcome(app *App, _ bool) string {
	p := app.panelByID(ui.PrimaryPanel)
	// Opening a result navigates the panel asynchronously: let the listing land first.
	screen := app.screen.(tcell.SimulationScreen)
	deadline := time.Now().Add(raceSlowdown * 3 * time.Second)
	for p.ListingPending && time.Now().Before(deadline) {
		for screen.HasPendingEvent() {
			if ev, ok := screen.PollEvent().(*tcell.EventInterrupt); ok {
				app.handleInterruptPayload(ev.Data())
				app.reconcileAfterEvent()
			}
		}
		time.Sleep(time.Millisecond)
	}
	var sel []string
	for s := range p.SelectedPaths {
		sel = append(sel, findKeyRel(app, s))
	}
	sort.Strings(sel)
	cur := ""
	if e, ok := p.CurrentEntry(); ok {
		cur = e.Name
	}
	return fmt.Sprintf("panel=%q cursor=%q selected=%v", findKeyRel(app, p.PathString()), cur, sel)
}

// findKeyDialog opens find (or jump) on the fixture; withSelection selects a directory in the
// panel first, which adds the search-only-selections checkbox row.
func findKeyDialog(name string, jump, withSelection bool) keyGoldenDialog {
	// The dialog is 76x18 on 80x24, one row taller with the selections row.
	ry, rh := 3, 18
	if withSelection {
		ry, rh = 2, 19
	}
	return keyGoldenDialog{
		name: name, rx: 2, ry: ry, rw: 76, rh: rh,
		setup: func(t *testing.T, dir string) *App {
			findKeyFixture(t, dir)
			app := newApp(t, newScreen(t, 80, 24), dir)
			findKeyRoots.Store(app, dir)
			t.Cleanup(func() { findKeyRoots.Delete(app) })
			app.model.Message = ""
			if withSelection {
				app.panelByID(ui.PrimaryPanel).AddSelection(filepath.Join(dir, "amber"))
			}
			if jump {
				app.findCtrl.OpenJumpDialog(ui.PrimaryPanel)
			} else {
				app.findCtrl.OpenDialog(ui.PrimaryPanel)
			}
			settleFind(t, app)
			return app
		},
		settle:  settleFind,
		state:   findKeyState,
		open:    func(app *App) bool { return app.model.FindDialog.Open },
		outcome: findKeyOutcome,
	}
}

// findQuery types s into the query as one token per rune ("Space" for a space).
func findQuery(s string) string {
	var toks []string
	for _, r := range s {
		if r == ' ' {
			toks = append(toks, "Space")
			continue
		}
		toks = append(toks, string(r))
	}
	return strings.Join(toks, " ")
}

func findKeyScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"walk-down", "Down Down Down Down Down Down Down Down Down Down Down Down Down Down Down Down Down Down"},
		{"page-and-ends", "PgDn PgDn PgUp Ctrl+End Ctrl+Home End Home"},
		{"query-then-nav", findQuery("note") + " Down Down Up"},
		{"query-edit", findQuery("lantren") + " Left Left Backspace Backspace " + findQuery("er") + " Home Delete End Ctrl+W"},
		{"query-no-match", findQuery("zzq") + " Down Up Enter"},
		{"tab-cycle", "Tab Tab Tab Tab Tab Tab Tab Tab Tab"},
		{"backtab-cycle", "Backtab Backtab Backtab Backtab Backtab"},
		{"right-into-depth-left-back", "Right Tab Left"},
		{"depth-row-nav", "Tab Down Right Right Right Right Left Up Down Down Up"},
		{"depth-typing", "Tab 2 Tab 0 Tab x Backspace Enter"},
		{"depth-digit-reindexes", "Tab 1"},
		{"buttons-nav", "Tab Tab Tab Tab Tab Tab Left Left Right Up Down"},
		{"space-on-options", "Tab Tab Space Right Space Right Space Right Space"},
		{"enter-on-options", "Tab Tab Enter Right Enter Right Enter"},
		{"mnemonics-from-query", "Alt+n Alt+y Alt+i Alt+v Alt+n"},
		{"mnemonics-from-option", "Tab Tab Alt+i Alt+y Alt+v"},
		{"mnemonic-selections-absent", "Alt+s"},
		{"plain-letters-on-option", "Tab Tab x o"},
		{"plain-c-on-option", "Tab Tab c"},
		{"space-ok", "Tab Tab Tab Tab Tab Tab Space"},
		{"space-on-ok-button", "Down Tab Tab Tab Space"},
		{"space-on-cancel-button", "Tab Tab Tab Right Space"},
		{"up-from-buttons", "Tab Tab Tab Up Up Down Down"},
		{"space-cancel", "Tab Tab Tab Tab Tab Tab Tab Space"},
		{"enter-on-file", findQuery("kestrel") + " Enter"},
		{"enter-on-dir", findQuery("juniper") + " Enter"},
		{"mark-and-ok", "Down Insert Insert Down Insert Enter"},
		{"mark-dir-then-child", findQuery("delta") + " Insert Down Insert"},
		{"alt-o", "Down Down Insert Alt+o"},
		{"alt-c", "Down Insert Alt+c"},
		{"alt-enter", "Down Alt+Enter"},
		{"esc", findQuery("har") + " Esc"},
		{"esc-from-option", "Tab Tab Esc"},
		{"hidden-toggle-reindexes", "Alt+i Down Alt+i"},
	}
}

func TestFindDialogKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, findKeyDialog("find_dialog", false, false), findKeyScripts())
}

func TestFindDialogSelectionsKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, findKeyDialog("find_dialog_selections", false, true), []simpleKeyScript{
		{"tab-cycle", "Tab Tab Tab Tab Tab Tab Tab Tab Tab Tab"},
		{"row-down-to-selections", "Tab Tab Right Down Up Down Down Up Up"},
		{"mnemonic-selections", "Alt+s Down Alt+s"},
		{"space-selections", "Tab Tab Tab Tab Tab Tab Space Space"},
		{"buttons-up", "Tab Tab Tab Tab Tab Tab Tab Up Tab Tab Left Up"},
	})
}

func TestFindJumpDialogKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, findKeyDialog("find_jump", true, false), []simpleKeyScript{
		{"walk-down", "Down Down Down Down Down"},
		{"tab-cycle", "Tab Tab Tab Tab Tab Tab Tab"},
		{"row-nav", "Tab Down Right Left Left Up"},
		{"mnemonics", "Alt+n Alt+y Alt+i Alt+v"},
		{"insert-ignored", "Down Insert Down"},
		{"query-enter", findQuery("quartz") + " Enter"},
		{"enter-first", "Enter"},
	})
}
