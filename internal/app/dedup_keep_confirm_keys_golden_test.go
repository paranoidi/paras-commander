package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the dedup Keep Directory confirmation. The dialog opens through the real
// flow: a duplicate in three folders, birch kept first, then keeping amber would drop that keep.
// The outcome records which copies end up kept and marked for deletion.

func dedupKeepFocusName(app *App) string {
	switch app.model.DedupKeepConfirm.Focus {
	case dialog.DedupKeepThis:
		return "this"
	case dlg.Cancel:
		return "cancel"
	}
	return "previous"
}

func dedupKeepSetNames(root string, set map[string]bool) string {
	var out []string
	for p, on := range set {
		if on {
			out = append(out, strings.TrimPrefix(p, root+"/"))
		}
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

func dedupKeepKeyDialog(name string) keyGoldenDialog {
	var roots perApp[string]
	selectDir := func(app *App, rel string) {
		for i, r := range app.model.DedupList {
			if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == rel {
				app.model.DedupView.Main.Selected = i
			}
		}
	}
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, _ string) *App {
			root := filepath.Join(t.TempDir(), "orchard")
			for _, d := range []string{"amber", "birch", "cedar"} {
				if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, d, "widget.txt"), []byte("dup"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			app := newApp(t, newScreen(t, 80, 24), root)
			app.model.Message = ""
			roots.set(t, app, root)
			app.openFindDuplicates()
			waitDedupDone(t, app)
			selectDir(app, "birch")
			dedupMarkKeep(app)
			selectDir(app, "amber")
			dedupMarkKeep(app)
			if !app.model.DedupKeepConfirm.Open {
				t.Fatal("keep confirmation did not open")
			}
			app.model.Message = ""
			return app
		},
		state: func(app *App) string { return "focus=" + dedupKeepFocusName(app) },
		open:  func(app *App) bool { return app.model.DedupKeepConfirm.Open },
		outcome: func(app *App, quit bool) string {
			root := roots.get(app)
			v := app.model.DedupView
			return fmt.Sprintf("quit=%v kept=[%s] marked=[%s]", quit, dedupKeepSetNames(root, v.Kept), dedupKeepSetNames(root, v.Marked))
		},
	}
}

func TestDedupKeepConfirmKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, dedupKeepKeyDialog("dedup_keep_confirm"), []simpleKeyScript{
		{"enter-previous", "Enter"},
		{"enter-this", "Right Enter"},
		{"enter-cancel", "Right Right Enter"},
		{"space", "Space"},
		{"alt-t", "Alt+t"},
		{"alt-p", "Right Alt+p"},
		{"alt-upper-t", "Alt+T"},
		{"alt-c", "Alt+c"},
		{"alt-enter", "Right Right Alt+Enter"},
		{"alt-unknown", "Alt+x Alt+q Right"},
		{"esc", "Right Esc"},
		{"plain-letters", "t p c x q"},
	})
}
