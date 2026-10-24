package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
)

// Render goldens for the small button/radio/checkbox dialogs. Each case draws one dialog on a
// blank simulation screen and dumps the 16 screen rows around the vertical center (every rune
// and style), so a dialog that grows, shrinks or shifts shows up as a diff. Focus positions are
// named by the per-dialog helpers below, which are the only code that knows how each state
// encodes focus.

const simpleGoldenRoot = "testdata/simple_dialogs"

type simpleGolden struct {
	name string
	w, h int
	draw func(screen tcell.Screen, layout Layout)
}

func dumpDialogBand(t *testing.T, c simpleGolden) string {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(c.w, c.h)
	c.draw(screen, Layout{Width: c.w, Height: c.h})
	screen.Show()
	top := max(c.h/2-8, 0)
	return tcelltest.DumpRect(screen, 0, top, c.w, min(16, c.h-top))
}

func checkGoldenFile(t *testing.T, dir, name, got string) {
	t.Helper()
	path := filepath.Join(dir, name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(dir, 0o755); err != nil {
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
}

func runSimpleGoldens(t *testing.T, dialogName string, cases []simpleGolden) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkGoldenFile(t, filepath.Join(simpleGoldenRoot, dialogName), c.name, dumpDialogBand(t, c))
		})
	}
}

func TestUndoConfirmGoldenRender(t *testing.T) {
	var cases []simpleGolden
	for _, focus := range []string{"yes", "no"} {
		st := undoGoldenState(focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("80x24_focus-%s", focus), 80, 24, func(s tcell.Screen, l Layout) {
			DrawUndoConfirmDialog(s, l, st, asciiDialogStyles())
		}})
	}
	runSimpleGoldens(t, "undo_confirm", cases)
}

func TestQuitConfirmGoldenRender(t *testing.T) {
	var cases []simpleGolden
	for _, variant := range []string{"default", "commands"} {
		for _, focus := range []string{"stay", "quit"} {
			st := quitGoldenState(focus, variant == "commands")
			cases = append(cases, simpleGolden{fmt.Sprintf("80x24_%s_focus-%s", variant, focus), 80, 24, func(s tcell.Screen, l Layout) {
				DrawQuitConfirmDialog(s, l, st, asciiDialogStyles())
			}})
		}
	}
	runSimpleGoldens(t, "quit_confirm", cases)
}

func TestListingFormatGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, format panel.ListFormat, focus string) {
		st := listingFormatGoldenState(format, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_format%d_focus-%s", w, h, format, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawListingFormatDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, f := range panel.ListFormatDialogRadios() {
		add(80, 24, f.Format, "radio0")
	}
	for _, focus := range []string{"radio1", "radio2", "ok", "cancel"} {
		add(80, 24, panel.ListFormatPerm, focus)
	}
	add(36, 24, panel.ListFormatBrief, "radio2")
	add(36, 24, panel.ListFormatMtime, "cancel")
	add(30, 24, panel.ListFormatMtime, "radio0") // too narrow to draw
	runSimpleGoldens(t, "listing_format", cases)
}

func TestCompareFilterGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, mode CompareFilterDialogMode, secondary bool, filter comparepkg.Filter, focus string) {
		st := compareFilterGoldenState(mode, secondary, filter, focus)
		name := fmt.Sprintf("%dx%d_mode%d_sec%v_filter%d_focus-%s", w, h, mode, secondary, filter, focus)
		cases = append(cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
			DrawCompareFilterDialog(s, l, st, asciiDialogStyles())
		}})
	}
	partial := comparepkg.FilterPrimaryOnly | comparepkg.FilterContentDiff
	for _, filter := range []comparepkg.Filter{comparepkg.FilterAll, comparepkg.FilterNone, partial} {
		for _, focus := range []string{"item0", "item3", "item5", "ok", "cancel"} {
			add(80, 24, CompareFilterModeCategory, false, filter, focus)
		}
	}
	for _, focus := range []string{"item1", "item4", "item5"} {
		add(80, 24, CompareFilterModeSelect, false, partial, focus)
		add(80, 24, CompareFilterModeUnselect, true, partial, focus)
	}
	add(80, 24, CompareFilterModeSelect, true, comparepkg.FilterAll, "ok")
	add(80, 24, CompareFilterModeUnselect, false, comparepkg.FilterNone, "cancel")
	add(34, 24, CompareFilterModeCategory, false, partial, "item2") // narrow: clamped to 30
	add(27, 24, CompareFilterModeCategory, false, partial, "item2") // too narrow to draw
	add(80, 11, CompareFilterModeCategory, false, partial, "item2") // too short to draw
	runSimpleGoldens(t, "compare_filter", cases)
}

func TestHostKeyGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, fp, focus string) {
		st := hostKeyGoldenState(fp, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_fp%d_focus-%s", w, h, len(fp), focus), w, h, func(s tcell.Screen, l Layout) {
			DrawHostKeyDialog(s, l, st, asciiDialogStyles())
		}})
	}
	short := "tulip+meadow/harbor+lantern=willow"
	long := "tulip+meadow/harbor+lantern=willow+copper/marble+anchor=orchard+velvet/silver+timber=ribbon"
	for _, focus := range []string{"accept", "save", "reject"} {
		add(80, 24, short, focus)
	}
	add(80, 24, long, "save")
	add(100, 30, long, "reject")
	add(60, 24, long, "accept") // width = W-4
	add(46, 24, short, "save")  // below the 44 floor: widened to W-2 at most
	add(40, 24, long, "reject")
	runSimpleGoldens(t, "hostkey", cases)
}

func TestMetaDialogGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h, n int, checked, focus string) {
		st := metaGoldenState(n, checked, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_n%d_checked-%s_focus-%s", w, h, n, checked, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawMetaDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, checked := range []string{"none", "mixed", "all"} {
		for _, focus := range []string{"item0", "item1", "item2", "ok", "cancel"} {
			add(80, 24, 3, checked, focus)
		}
	}
	add(80, 24, 1, "mixed", "item0")
	add(80, 24, 5, "mixed", "item4")
	add(50, 24, 3, "mixed", "item1") // width = W-4
	add(32, 24, 3, "all", "ok")      // narrow: 28 wide is the smallest
	add(80, 24, 3, "mixed", "item1")
	add(80, 7, 3, "mixed", "item2") // too short: height clamps to 5
	runSimpleGoldens(t, "meta", cases)
}

func TestStashRestoreGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, focus string) {
		st := stashRestoreGoldenState(focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_focus-%s", w, h, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawStashRestoreDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, focus := range []string{"replace", "merge", "drop-stash", "drop-all", "cancel"} {
		add(80, 24, focus)
	}
	add(120, 30, "merge")
	add(70, 24, "drop-all") // 66 wide, below the 68 floor: widened to W-2 at most
	add(60, 24, "cancel")
	runSimpleGoldens(t, "stash_restore", cases)
}

func TestDedupReturnGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, scope, focus string) {
		st := dedupReturnGoldenState(scope, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_scope-%s_focus-%s", w, h, scope, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawDedupReturnDialog(s, l, st, "/home/user/orchard/meadow", 12, 3<<20, asciiDialogStyles(), "/home/user")
		}})
	}
	for _, scope := range []string{"none", "one", "many"} {
		for _, focus := range []string{"first", "second", "cancel"} {
			add(80, 24, scope, focus)
		}
	}
	add(60, 24, "none", "second")
	add(50, 24, "many", "first")
	runSimpleGoldens(t, "dedup_return", cases)
}
