package app

import (
	"fmt"
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the find-duplicates progress modal in each phase. The snapshot is
// injected (no scan session runs), so the confirm gate's OK leaves the dialog open and the
// hashing ETA stays deterministic.

func dedupProgressFocusName(app *App) string {
	// The index of the focused button: only the confirm gate has two (OK, Cancel).
	idx := 0
	if app.model.DedupSnapshot.Phase == comparepkg.DedupAwaitConfirm && app.model.DedupProgressDialog.Focus == dlg.Cancel {
		idx = 1
	}
	return fmt.Sprintf("focus=%d", idx)
}

func dedupProgressKeyDialog(name string, phase comparepkg.DedupPhase) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			snap := comparepkg.DedupSnapshot{Root: pathloc.MustParse("/srv/meadow/orchard"), Phase: phase, Walked: 12}
			if phase != comparepkg.DedupWalking {
				snap.HashBytesTotal = 5 << 30
			}
			if phase == comparepkg.DedupHashing {
				snap.Current, snap.Hashed, snap.HashTotal, snap.HashedBytes = "media/archive", 1, 4, 1<<30
			}
			app.model.DedupSnapshot = snap
			app.model.DedupProgressDialog = dialog.DedupProgressDialogState{Open: true}
			t.Cleanup(app.dedupCtrl.Close)
			return app
		},
		state: dedupProgressFocusName,
		open:  func(app *App) bool { return app.model.DedupProgressDialog.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("view=%v progress=%v quit=%v", app.model.ViewMode, app.model.DedupProgressDialog.Open, quit)
		},
	}
}

func TestDedupProgressKeyScripts(t *testing.T) {
	common := []simpleKeyScript{
		{"left-right", "Left Right Right Right Left Left Left"},
		{"up-down", "Up Down Right Up Down Left Down"},
		{"tab", "Tab Tab Backtab Right Tab Backtab"},
		{"enter", "Enter"},
		{"enter-second", "Right Enter"},
		{"space", "Space Right Space"},
		{"esc", "Right Esc"},
		{"esc-first", "Esc"},
		{"alt-c", "Alt+c"},
		{"alt-o", "Alt+o Down"},
		{"alt-upper", "Alt+C"},
		{"plain-letters", "o c y n x"},
		{"alt-enter-swallowed", "Alt+Enter Alt+x"},
		{"home-end-pgdn", "Home End PgUp PgDn"},
	}
	runKeyGoldenScripts(t, dedupProgressKeyDialog("dedup_progress_walking", comparepkg.DedupWalking), common)
	runKeyGoldenScripts(t, dedupProgressKeyDialog("dedup_progress_confirm", comparepkg.DedupAwaitConfirm), common)
	runKeyGoldenScripts(t, dedupProgressKeyDialog("dedup_progress_hashing", comparepkg.DedupHashing), common)
}
