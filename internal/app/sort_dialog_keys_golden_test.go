package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

var updateKeyGolden = flag.Bool("update", false, "rewrite dialog key-script golden files")

const sortKeyGoldenDir = "testdata/sort_dialog_keys"

// goldenKeyEvent parses a script token: a named key, "Alt+<letter>", or a single plain rune.
func goldenKeyEvent(t *testing.T, tok string) *tcell.EventKey {
	t.Helper()
	switch tok {
	case "Down":
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case "Up":
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case "Left":
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	case "Right":
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	case "Tab":
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	case "Backtab":
		return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift)
	case "Enter":
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	case "Esc":
		return tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	case "Space":
		return tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)
	case "Alt+Enter":
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt)
	case "F9":
		return tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone)
	case "Backspace":
		return tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone)
	case "Delete":
		return tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone)
	case "Home":
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
	case "End":
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
	case "Insert":
		return tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)
	case "F8":
		return tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)
	case "PgUp":
		return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
	case "PgDn":
		return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
	case "Ctrl+Home":
		return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModCtrl)
	case "Ctrl+End":
		return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl)
	case "F5":
		return tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)
	case "Ctrl+W":
		return tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone)
	case "Ctrl+U":
		return tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone)
	case "Ctrl+K":
		return tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone)
	case "Ctrl+L":
		return tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone)
	case "Ctrl+Y":
		return tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModNone)
	case "Alt+Backspace":
		return tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModAlt)
	case "Alt+Insert":
		return tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModAlt)
	case "F3":
		return tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone)
	case "F2":
		return tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone)
	case "F4":
		return tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone)
	case "F7":
		return tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone)
	case "Shift+F8":
		return tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModShift)
	case "Shift+Left":
		return tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift)
	case "Shift+Right":
		return tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift)
	}
	if r, ok := strings.CutPrefix(tok, "Alt+"); ok && len([]rune(r)) == 1 {
		return tcell.NewEventKey(tcell.KeyRune, []rune(r)[0], tcell.ModAlt)
	}
	if len([]rune(tok)) == 1 {
		return tcell.NewEventKey(tcell.KeyRune, []rune(tok)[0], tcell.ModNone)
	}
	t.Fatalf("bad script token %q", tok)
	return nil
}

func sortStateSummary(app *App) string {
	st := app.model.SortDialog
	return fmt.Sprintf("dialog: focus=%q mode=%s meta=%q reverse=%v dirsFirst=%v diskUsage=%v",
		dlg.Focused(dialog.SortDialogView(st), st.Focus), st.SortMode, st.MetaColumn, st.SortReverse, st.DirectoriesFirst, st.DiskUsageIdleSizeSort)
}

// runSortKeyScript opens the sort dialog with n meta columns, feeds the tokens through
// App.handleKey and returns a transcript: the dialog state after every key. Once the dialog closes the transcript records whether it applied
// (the "Sort:" toast) or cancelled, along with the panel's resulting sort state.
func runSortKeyScript(t *testing.T, metas int, keys []string) string {
	t.Helper()
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	titles := []string{"duration", "bitrate", "codec", "resolution"}
	var cols []ui.MetaColumnState
	for _, name := range titles[:metas] {
		cols = append(cols, ui.MetaColumnState{EntryName: name, ColumnTitle: name})
	}
	app.model.MetaResults[ui.PrimaryPanel] = cols
	app.model.ActivePanel = ui.PrimaryPanel
	app.model.Message = ""
	app.openSortDialog()

	var sb strings.Builder
	dump := func(label string) {
		app.render()
		fmt.Fprintf(&sb, "== %s ==\n%s\n", label, sortStateSummary(app))
	}
	dump("open")
	for _, tok := range keys {
		app.handleKey(goldenKeyEvent(t, tok))
		if !app.model.SortDialog.Open {
			outcome := "cancelled"
			if strings.HasPrefix(app.model.Message, "Sort:") {
				outcome = "applied"
			}
			s := app.model.Primary.Sort
			fmt.Fprintf(&sb, "== %s ==\nCLOSED: %s\n%s\npanel sort: mode=%s meta=%q reverse=%v dirsFirst=%v diskUsage=%v\n",
				tok, outcome, sortStateSummary(app), s.Mode, s.MetaColumn, s.Reverse, s.DirectoriesFirst, s.DiskUsageIdleSizeSort)
			return sb.String()
		}
		dump(tok)
	}
	return sb.String()
}

type sortKeyScript struct {
	name  string
	metas []int
	keys  string // space-separated tokens
}

var sortKeyScripts = []sortKeyScript{
	{"space-radios", []int{4}, "Down Space Down Down Space Right Space Down Space Left Space"},
	{"enter-radios", []int{2}, "Down Down Enter Right Enter Down Left Enter"},
	{"space-checkboxes", []int{0}, "Tab Space Down Space Down Space Space Up Space"},
	{"enter-checkboxes", []int{0}, "Tab Enter Down Enter Down Enter Enter"},
	{"space-ok", []int{4}, "Right Down Space Tab Tab Space"},
	{"enter-ok", []int{4}, "Down Space Tab Tab Enter"},
	{"space-cancel", []int{4}, "Down Space Tab Tab Right Space"},
	{"enter-cancel", []int{4}, "Tab Space Tab Right Enter"},
	{"mnemonics", []int{4}, "Alt+n Alt+e Alt+s Alt+m Alt+u Alt+r Alt+d Alt+u Alt+r Alt+d"},
	{"mnemonics-upper", []int{0}, "Alt+E Alt+S Alt+M Alt+U Alt+R Alt+D Alt+N"},
	{"mnemonics-from-meta-focus", []int{4}, "Right Right Down Alt+s Alt+u Down Alt+n"},
	{"mnemonic-unknown", []int{0}, "Alt+x Alt+q Down"},
	{"mnemonic-meta-then-apply", []int{4}, "Right Down Space Alt+r Tab Tab Enter"},
	{"alt-o", []int{4}, "Down Alt+r Alt+o"},
	{"alt-c", []int{4}, "Down Alt+r Alt+c"},
	{"plain-o", []int{4}, "Down Space Tab Space o"},
	{"plain-c", []int{4}, "Down Space Tab Space c"},
	{"plain-upper-o", []int{0}, "Tab Space O"},
	{"plain-upper-c", []int{0}, "Tab Space C"},
	{"plain-other-letters", []int{4}, "n s x u r d q"},
	{"esc", []int{4}, "Down Space Tab Space Esc"},
	{"esc-immediately", []int{0}, "Esc"},
	{"space-then-enter-same-radio", []int{4}, "Right Right Space Enter Enter"},
}

func TestSortDialogKeyScripts(t *testing.T) {
	for _, sc := range sortKeyScripts {
		for _, metas := range sc.metas {
			name := fmt.Sprintf("%s_meta%d", sc.name, metas)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				got := runSortKeyScript(t, metas, strings.Fields(sc.keys))
				path := filepath.Join(sortKeyGoldenDir, name+".golden")
				if *updateKeyGolden {
					if err := os.MkdirAll(sortKeyGoldenDir, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read golden (run with -update to create): %v", err)
				}
				if string(want) != got {
					t.Errorf("%s differs from golden %s\n--- got ---\n%s\n--- want ---\n%s", name, path, got, want)
				}
			})
		}
	}
}
