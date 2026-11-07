package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// metaAlertRow draws a panel whose copies column numbers state.Entries from 1 and returns the
// style of the cell holding value on the given row.
func metaAlertRow(t *testing.T, state panel.State, row int, value rune, ctx PanelContext) tcell.Style {
	t.Helper()
	results := map[string]string{}
	for i, e := range state.Entries {
		results[e.Path] = string(rune('1' + i))
	}
	return metaAlertRowWith(t, state, results, row, value, ctx)
}

func metaAlertRowWith(t *testing.T, state panel.State, results map[string]string, row int, value rune, ctx PanelContext) tcell.Style {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 12
	screen.SetSize(width, height)

	ctx.PanelID, ctx.ActivePanel = PrimaryPanel, PrimaryPanel
	ctx.SyncDriverPanelID, ctx.QuickViewDriverPanelID = -1, -1
	drawPanel(screen, Rect{Width: width, Height: height}, state, PanelStyleConfig{Styles: theme.Default()}, ctx, PanelDisplayConfig{
		CarouselLayout: panelcarousel.DefaultLayout(),
		MetaColumns: []MetaColumnState{{
			EntryName: "copies", ColumnTitle: "Copies", Results: results,
			Alerts: panel.MustMetaAlerts([]string{">1"}, []string{">2"}),
		}},
	})

	for x := range width {
		ch, st, _ := screen.Get(x, 2+row)
		if ch == string(value) {
			return st
		}
	}
	t.Fatalf("value %q not on row %d", value, row)
	return tcell.StyleDefault
}

func metaAlertState(names ...string) panel.State {
	state := panel.State{Path: pathloc.MustParse("/tmp"), ListFormat: panel.ListFormatBrief}
	for _, n := range names {
		state.Entries = append(state.Entries, localfs.Entry{Name: n, Path: "/tmp/" + n, Type: localfs.EntryFile})
	}
	return state
}

func assertMetaFG(t *testing.T, label string, st, wantStyle tcell.Style) {
	t.Helper()
	got, _, gotAttrs := st.Decompose()
	want, _, wantAttrs := wantStyle.Decompose()
	if got != want || gotAttrs != wantAttrs {
		t.Fatalf("%s: fg %v attrs %v, want fg %v attrs %v", label, got, gotAttrs, want, wantAttrs)
	}
}

func TestMetaAlertColorsCells(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	state := metaAlertState("harbor", "meadow", "thistle")
	state.Cursor = 2
	ctx := PanelContext{}
	assertMetaFG(t, "no alert", metaAlertRow(t, state, 0, '1', ctx), styles.PanelRowInfo)
	assertMetaFG(t, "warn", metaAlertRow(t, state, 1, '2', ctx), styles.PanelMetaWarn)

	state.Cursor = 0
	assertMetaFG(t, "critical", metaAlertRow(t, state, 2, '3', ctx), styles.PanelMetaCritical)
}

func TestMetaAlertUsesCursorIconColorOnActiveCursor(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	wantFG, ok := styles.PanelFileIconFG["panel.active.row.cursor"]
	if !ok {
		t.Fatal("default theme missing panel.active.row.cursor icon")
	}
	state := metaAlertState("harbor", "meadow", "thistle")
	state.Cursor = 2
	st := metaAlertRow(t, state, 2, '3', PanelContext{FileListActive: true, CursorRowActive: true})
	fg, bg, _ := st.Decompose()
	_, wantBG, _ := styles.PanelCursorActive.Decompose()
	if fg != wantFG || bg != wantBG {
		t.Fatalf("fg %v bg %v, want fg %v bg %v", fg, bg, wantFG, wantBG)
	}
}

func TestMetaAlertKeepsColorOnInactiveCursor(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	state := metaAlertState("harbor", "meadow")
	state.Cursor = 1
	st := metaAlertRow(t, state, 1, '2', PanelContext{})
	assertMetaFG(t, "inactive cursor", st, styles.PanelMetaWarn)
	_, bg, _ := st.Decompose()
	_, wantBG, _ := styles.PanelCursorInactive.Decompose()
	if bg != wantBG {
		t.Fatalf("bg %v, want the cursor bar's %v", bg, wantBG)
	}
}

func TestMetaAlertOffWhileChromeBlocked(t *testing.T) {
	t.Parallel()
	state := metaAlertState("harbor", "meadow")
	st := metaAlertRow(t, state, 1, '2', PanelContext{ChromeBlocked: true})
	assertMetaFG(t, "blocked", st, theme.Default().PanelRowInfo)
}

func TestMetaAlertStaysOnColumnWithWideName(t *testing.T) {
	t.Parallel()
	name := "港口草地" // two cells per rune: spans would drift by the rune/cell difference
	if runewidth.StringWidth(name) == len([]rune(name)) {
		t.Fatal("test requires a wide-character name")
	}
	state := metaAlertState(name, "harbor")
	state.Cursor = 1
	st := metaAlertRow(t, state, 0, '1', PanelContext{})
	assertMetaFG(t, "no alert", st, theme.Default().PanelRowInfo)

	state = metaAlertState("harbor", name)
	state.Cursor = 0
	assertMetaFG(t, "wide name", metaAlertRow(t, state, 1, '2', PanelContext{}), theme.Default().PanelMetaWarn)
}

func TestMetaAlertInTreeLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	child := filepath.Join(root, "meadow", "harbor.txt")
	if err := os.Mkdir(filepath.Dir(child), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	state.SetListLayout(panel.ListLayoutTree, 10)
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatal(err)
	}
	st := metaAlertRowWith(t, state, map[string]string{child: "2"}, 1, '2', PanelContext{})
	assertMetaFG(t, "nested row", st, theme.Default().PanelMetaWarn)
}
