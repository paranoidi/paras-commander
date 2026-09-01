package dialog

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func TestFindDialogTitleIndexingWorkers(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	st := FindDialogState{Indexing: true, IndexedCount: 123, WalkWorkers: 4}
	got := findDialogTitle(st, th)
	icon := string(th.SymbolMenuJob("scanning"))
	want := "Find (123…) 4 " + icon
	if got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}

func TestFindDialogTitleIndexingCompactCount(t *testing.T) {
	t.Parallel()
	st := FindDialogState{Indexing: true, IndexedCount: 1_240_000, WalkWorkers: 4}
	got := findDialogTitle(st, theme.Default())
	if !strings.HasPrefix(got, "Find (1.24M") {
		t.Fatalf("title = %q, want 1.24M prefix", got)
	}
}

func TestFormatIndexedCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.00K"},
		{1234, "1.23K"},
		{1_000_000, "1.00M"},
		{1_240_000, "1.24M"},
		{20_487_177, "20.49M"},
	}
	for _, tc := range cases {
		if got := formatIndexedCount(tc.n); got != tc.want {
			t.Fatalf("formatIndexedCount(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFindDialogTitleIndexingNoWorkersYet(t *testing.T) {
	t.Parallel()
	st := FindDialogState{Indexing: true, IndexedCount: 5}
	got := findDialogTitle(st, theme.Default())
	if got != "Find (5…)" {
		t.Fatalf("title = %q, want %q", got, "Find (5…)")
	}
}

func TestFindDialogTitleDone(t *testing.T) {
	t.Parallel()
	st := FindDialogState{IndexDone: true, IndexedCount: 42}
	got := findDialogTitle(st, theme.Default())
	if got != "Find (42)" {
		t.Fatalf("title = %q, want %q", got, "Find (42)")
	}
	if strings.Contains(got, "workers") {
		t.Fatalf("done title must not mention workers: %q", got)
	}
}

func TestFindDialogTitleIndexingNoWorkerSuffixWord(t *testing.T) {
	t.Parallel()
	st := FindDialogState{Indexing: true, IndexedCount: 1, WalkWorkers: 2}
	got := findDialogTitle(st, theme.Default())
	if strings.Contains(got, "workers") {
		t.Fatalf("title must not contain workers: %q", got)
	}
}

func TestDrawFindDialogShowsPinGlyphOnlyForPinnedRow(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	layout := Layout{Width: 80, Height: 24}
	styles := theme.Default()
	state := FindDialogState{
		Open:     true,
		RootPath: "/root",
		Entries: []FindEntry{
			{RelLine: "pinned.txt"},
			{RelLine: "other.txt"},
		},
		Ranked:   []int{0, 1},
		Selected: -1,
	}

	pinnedAbs := state.Entries[0].AbsPath(state.RootPath)
	rowMarks := func(absPath string) RowMarks {
		return RowMarks{Pinned: absPath == pinnedAbs}
	}

	DrawFindDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil, "", rowMarks)

	width, height, _, ok := FindDialogMetrics(layout, false)
	if !ok {
		t.Fatal("FindDialogMetrics: want ok")
	}
	rect := draw.CenteredDialogRect(layout, width, height)
	checkboxRows := 1
	sepAfterCheckbox := rect.Y + 3 + checkboxRows
	listTop := sepAfterCheckbox + 1

	pinGlyph := []rune(styles.SymbolPin())[0]
	rowHasGlyph := func(y int) bool {
		for x := rect.X + 1; x < rect.X+rect.Width-1; x++ {
			str, _, _ := screen.Get(x, y)
			r, _ := utf8.DecodeRuneInString(str)
			if r == pinGlyph {
				return true
			}
		}
		return false
	}
	if !rowHasGlyph(listTop) {
		t.Error("expected pin glyph on pinned row")
	}
	if rowHasGlyph(listTop + 1) {
		t.Error("did not expect pin glyph on unpinned row")
	}
}
