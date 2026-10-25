package ui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the find dialog, painted through PaintFindDialog (the path the app and its
// partial repaint use) so row devicons, row marks and the selection-size rule label are covered.
// Each case dumps the whole screen, every rune and style.

var updateFindGolden = flag.Bool("update", false, "rewrite find dialog golden files")

const findGoldenDir = "testdata/find_dialog"

const findGoldenRoot = "/willow"

// findGoldenEntries is a fixed corpus of directories and files with assorted extensions, so the
// devicon column shows several icon kinds.
func findGoldenEntries() []dialog.FindEntry {
	words := []string{"amber", "birch", "cinder", "delta", "ember", "fable", "glacier", "harbor", "ivory", "juniper", "kestrel", "lantern"}
	exts := []string{".go", ".md", ".png", ".txt", ".toml", ".mp4", ".zip", ""}
	var out []dialog.FindEntry
	for i, w := range words {
		out = append(out, dialog.FindEntry{RelLine: w, IsDir: true, Type: localfs.EntryDirectory})
		for j := range 3 {
			ext := exts[(i+j)%len(exts)]
			name := fmt.Sprintf("%s/%s_%s%s", w, words[(i+j+1)%len(words)], words[(i+2*j+3)%len(words)], ext)
			out = append(out, dialog.FindEntry{RelLine: name, Type: localfs.EntryFile, Size: int64(1000 * (i + j + 1))})
		}
	}
	out = append(out, dialog.FindEntry{
		RelLine: "harbor/orchard/quartz/meadow/thistle/willowbrook_lantern_overview_notes_final.md",
		Type:    localfs.EntryFile, Size: 4096,
	})
	return out
}

func findGoldenState(mut func(st *dialog.FindDialogState)) *dialog.FindDialogState {
	entries := findGoldenEntries()
	st := &dialog.FindDialogState{
		Open: true, RootPath: findGoldenRoot, Entries: entries,
		IndexDone: true, IndexedCount: len(entries),
	}
	for i := range entries {
		st.Ranked = append(st.Ranked, i)
	}
	st.RebuildPathIndex()
	st.PathMeta = func(absPath string) (bool, int64, bool) {
		idx, ok := st.PathIndexLookup(absPath)
		if !ok {
			return false, 0, false
		}
		e := st.Entries[idx]
		return e.IsDir, e.Size, true
	}
	if mut != nil {
		mut(st)
	}
	return st
}

// rankQuery keeps only entries whose RelLine contains needle and records its match range.
func rankQuery(st *dialog.FindDialogState, needle string) {
	st.Query = needle
	st.QueryCursor = len([]rune(needle))
	st.Ranked = nil
	st.MatchRanges = map[int][]search.Range{}
	for i, e := range st.Entries {
		for k := 0; k+len(needle) <= len(e.RelLine); k++ {
			if e.RelLine[k:k+len(needle)] == needle {
				st.Ranked = append(st.Ranked, i)
				st.MatchRanges[i] = []search.Range{{Start: k, End: k + len(needle)}}
				break
			}
		}
	}
}

type findGolden struct {
	name     string
	w, h     int
	nerdfont bool
	icons    bool
	pinned   []PinnedItem
	jobs     []JobPathMark
	state    func(st *dialog.FindDialogState)
}

// findFocusOrder lists the dialog's widgets in focus order; golden names number them by position
// (0 = the query, which also lights the list cursor).
func findFocusOrder(selections bool) []dlg.ID {
	ids := []dlg.ID{dialog.FindQuery, dialog.FindDepth, dialog.FindOnlyDirs, dialog.FindOnlyFiles, dialog.FindHidden, dialog.FindStayOnVol}
	if selections {
		ids = append(ids, dialog.FindSelections)
	}
	return append(ids, dlg.OK, dlg.Cancel)
}

func findFocusCases() []findGolden {
	var out []findGolden
	for _, sel := range []bool{false, true} {
		for f, id := range findFocusOrder(sel) {
			out = append(out, findGolden{
				name: fmt.Sprintf("125x40_sel%v_focus%d", sel, f), w: 125, h: 40, icons: true,
				state: func(st *dialog.FindDialogState) {
					st.ShowSearchSelectionsOption = sel
					st.Focus = id
					st.Selected = 2
				},
			})
		}
	}
	return out
}

func findGoldenCases() []findGolden {
	cases := findFocusCases()
	cases = append(cases,
		findGolden{name: "80x24_empty_query", w: 80, h: 24, icons: true},
		findGolden{name: "80x24_noicons", w: 80, h: 24},
		findGolden{name: "80x24_nerdfont", w: 80, h: 24, icons: true, nerdfont: true},
		findGolden{name: "80x24_typed_query", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			rankQuery(st, "harbor")
			st.QueryScroll = 0
		}},
		findGolden{name: "80x24_no_matches", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			rankQuery(st, "zzzz")
		}},
		findGolden{name: "80x24_scrolled_middle", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.Selected = 20
			st.ListScroll = 17
		}},
		findGolden{name: "80x24_scrolled_bottom", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.Selected = len(st.Ranked) - 1
			st.ListScroll = len(st.Ranked) - 10
		}},
		findGolden{name: "80x24_long_query_scrolled", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.Query = "harbor orchard quartz meadow thistle willowbrook lantern overview notes final"
			st.QueryCursor = len([]rune(st.Query))
			st.QueryScroll = 30
			st.Ranked = st.Ranked[len(st.Ranked)-1:]
		}},
		findGolden{name: "80x24_depth_typed", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.DepthInput = "3"
			st.MaxDepth = 3
			st.Focus = dialog.FindDepth
		}},
		findGolden{name: "80x24_options_on", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.OnlyFiles = true
			st.IncludeHidden = true
			st.StayOnCurrentVolume = true
			st.ShowSearchSelectionsOption = true
			st.SearchOnlySelections = true
		}},
		findGolden{name: "80x24_marked_with_size", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.MarkedPaths = map[string]bool{}
			for _, i := range []int{1, 2, 6} {
				st.MarkedPaths[st.Entries[i].AbsPath(findGoldenRoot)] = true
			}
			st.Selected = 2
		}},
		findGolden{name: "80x24_marked_cursor_on_buttons", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.MarkedPaths = map[string]bool{st.Entries[1].AbsPath(findGoldenRoot): true}
			st.Focus = dlg.OK
		}},
		findGolden{
			name: "80x24_row_marks", w: 80, h: 24, icons: true,
			pinned: []PinnedItem{{Path: filepath.Join(findGoldenRoot, "amber")}, {Path: filepath.Join(findGoldenRoot, "birch")}},
			jobs:   []JobPathMark{{ID: "job-quartz", Type: "copy", Status: "running", Sources: []string{filepath.Join(findGoldenRoot, "amber")}}},
		},
		findGolden{name: "80x24_jump_mode", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.JumpMode = true
			st.Focus = dialog.FindOnlyDirs // hidden in jump mode
		}},
		findGolden{name: "80x24_indexing", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.IndexDone = false
			st.Indexing = true
			st.IndexedCount = 12
			st.Depth, st.LevelDone, st.LevelTotal = 2, 3, 8
		}},
		findGolden{name: "80x24_index_error", w: 80, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			st.IndexErr = "permission denied"
		}},
		findGolden{name: "58x24_narrow", w: 58, h: 24, icons: true, state: func(st *dialog.FindDialogState) {
			rankQuery(st, "an")
		}},
		findGolden{name: "60x20_narrow_selections", w: 60, h: 20, icons: true, state: func(st *dialog.FindDialogState) {
			st.ShowSearchSelectionsOption = true
			st.Focus = dialog.FindSelections
		}},
		findGolden{name: "80x14_short", w: 80, h: 14, icons: true},
		findGolden{name: "80x13_short_selections", w: 80, h: 13, icons: true, state: func(st *dialog.FindDialogState) {
			st.ShowSearchSelectionsOption = true
		}},
		findGolden{name: "57x24_too_narrow", w: 57, h: 24, icons: true},
		findGolden{name: "80x10_too_short", w: 80, h: 10, icons: true},
	)
	return cases
}

func TestFindDialogGoldenRender(t *testing.T) {
	for _, c := range findGoldenCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			t.Cleanup(screen.Fini)
			screen.SetSize(c.w, c.h)
			styles := theme.Default()
			styles.UseNerdfontIcons = c.nerdfont
			st := findGoldenState(c.state)
			PaintFindDialog(screen, Layout{Width: c.w, Height: c.h}, st, styles, c.icons, uiscrollbar.StyleThumb, nil, c.pinned, c.jobs)
			screen.Show()
			got := tcelltest.DumpRect(screen, 0, 0, c.w, c.h)
			path := filepath.Join(findGoldenDir, c.name+".golden")
			if *updateFindGolden {
				if err := os.MkdirAll(findGoldenDir, 0o755); err != nil {
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
				t.Errorf("%s differs from golden %s\n--- got ---\n%s\n--- want ---\n%s", c.name, path, got, want)
			}
		})
	}
}
