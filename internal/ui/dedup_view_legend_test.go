package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func dedupBottomRowText(screen tcell.Screen, y, w int) string {
	var b strings.Builder
	for x := range w {
		r, _, _ := screen.Get(x, y)
		b.WriteString(r)
	}
	return b.String()
}

func TestDrawDedupViewBottomBorderExplainsCursorRowIcons(t *testing.T) {
	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "meadow/lantern.txt", Abs: pathloc.MustParse("/scan/root/meadow/lantern.txt")},
				{Rel: "lantern.txt", Abs: pathloc.MustParse("/scan/root/lantern.txt")},
			},
		}},
	}
	base := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Marked:      map[string]bool{"/scan/root/meadow/lantern.txt": true},
		MarkedCount: 1,
	}
	list, _ := DedupRowsFromSnapshot(snap, base)
	dirIdx, fileIdx := -1, -1
	for i, row := range list {
		switch {
		case row.Value.Kind == DedupRowDir && row.Value.DirRel == "meadow":
			dirIdx = i
		case row.Value.Kind == DedupRowFile && row.Value.AbsKey == "/scan/root/lantern.txt":
			fileIdx = i
		}
	}
	if dirIdx < 0 || fileIdx < 0 {
		t.Fatalf("rows = %+v", list)
	}

	for _, tc := range []struct {
		name  string
		idx   int
		focus bool
		want  string
	}{
		{"marked dir row", dirIdx, false, "contains marked files"},
		{"plain file row", fileIdx, false, ""},
		{"unfocused pane", dirIdx, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			defer screen.Fini()
			screen.SetSize(80, 16)
			view := base
			view.Main = DedupPane{Selected: tc.idx}
			view.SourceRow = tc.idx
			view.FocusCopies = tc.focus
			drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal, nil)
			got := dedupBottomRowText(screen, 13, 40)
			if tc.want == "" {
				if strings.Contains(got, "contains") {
					t.Fatalf("bottom row %q, want no legend", got)
				}
				return
			}
			// Border-overlaid labels keep one space of padding on each side.
			wantPadded := "─ " + string(styles.IconFilelistSelectionSubtree()) + " " + tc.want + " ─"
			if !strings.Contains(got, wantPadded) {
				t.Fatalf("bottom row %q, want %q", got, wantPadded)
			}
		})
	}
}

func TestDedupRowLegendListsDirDupAndHiddenIcons(t *testing.T) {
	styles := theme.Default()
	d := DedupRowData{Kind: DedupRowDir, DirRel: "orchard", DirDup: comparepkg.DirExact, DirHidden: true}
	p := dedupPaneParams{TwinDirs: map[string]bool{}}
	got := dedupRowLegend(styles, p, comparepkg.DedupSnapshot{}, d, false)
	want := string(styles.IconDedupFullDir()) + " exact duplicate folder, " + string(styles.IconDedupHidden()) + " has skipped items"
	if got != want {
		t.Fatalf("legend %q, want %q", got, want)
	}
	// The related-copy + is never explained.
	if got := dedupRowLegend(styles, p, comparepkg.DedupSnapshot{}, DedupRowData{Kind: DedupRowDir, DirRel: "x"}, true); !strings.Contains(got, "contains copies") {
		t.Fatalf("within legend %q", got)
	}
	file := DedupRowData{Kind: DedupRowFile}
	if got := dedupRowLegend(styles, p, comparepkg.DedupSnapshot{}, file, true); got != "" {
		t.Fatalf("related + legend %q, want empty", got)
	}
}
