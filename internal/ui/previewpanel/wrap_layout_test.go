package previewpanel

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func internalState(text string, gutter int) State {
	st := State{Source: SourceInternalHighlighted, GutterWidth: gutter}
	st.SetHighlightedCells(AnsiStyledCells(text, tcell.StyleDefault))
	return st
}

func cellsText(cells []AnsiCell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.R)
	}
	return b.String()
}

func TestWrapRowsTrailingNewline(t *testing.T) {
	for _, tc := range []struct {
		name   string
		text   string
		gutter int
		want   []string
	}{
		{"empty body", "", 0, []string{""}},
		{"plain trailing newline keeps an empty row", "ab\n", 0, []string{"ab", ""}},
		{"gutter trailing newline adds no row", "1 ab\n", 2, []string{"1 ab"}},
		{"blank lines stay rows", "a\n\nb", 0, []string{"a", "", "b"}},
		{"soft wrap", "abcdefg", 0, []string{"abc", "def", "g"}},
		{"soft wrap behind gutter", "1 abcdef", 2, []string{"1 ab", "  cd", "  ef"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			width := 3
			if tc.gutter > 0 {
				width = 4
			}
			st := internalState(tc.text, tc.gutter)
			rows := st.WrappedRows(width, tcell.StyleDefault)
			if len(rows) != len(tc.want) {
				t.Fatalf("rows = %d, want %d", len(rows), len(tc.want))
			}
			for i, want := range tc.want {
				if got := cellsText(rows[i]); got != want {
					t.Fatalf("row %d = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestWrapWidthNotWiderThanGutterTerminates(t *testing.T) {
	st := internalState("12 walnut\n", 3)
	done := make(chan int, 1)
	go func() { done <- st.WrappedLineCount(3, tcell.StyleDefault) }()
	select {
	case n := <-done:
		if n < 2 {
			t.Fatalf("rows = %d, want the line split over several rows", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("wrapping at gutter width did not terminate")
	}
}

func TestWrapLayoutAliasesBodyCells(t *testing.T) {
	st := internalState(strings.Repeat("anchor ", 100), 0)
	st.EnsureWrapLayout(20, tcell.StyleDefault)
	if &st.layout.cells[0] != &st.HighlightedCells[0] {
		t.Fatal("layout must index the body, not copy it")
	}
}

func TestSearchStepKeepsLayout(t *testing.T) {
	st := internalState("foo bar foo\n", 0)
	st.Search = SearchState{Active: true, Matches: []SearchMatch{{Start: 0, End: 3}, {Start: 8, End: 11}}}
	st.EnsureWrapLayout(40, tcell.StyleDefault)
	before := st.layout
	st.Search.Current = 1
	st.CancelSearch()
	st.StartSearch()
	st.EnsureWrapLayout(40, tcell.StyleDefault)
	if st.layout != before {
		t.Fatal("search state changes must not rebuild the layout")
	}
}

func TestSearchOverlaySpansSoftWrapAndStylesIndent(t *testing.T) {
	base := tcell.StyleDefault
	hit := base.Foreground(tcell.ColorYellow)
	st := internalState("1 abcdef", 2)
	// "bcde" crosses the soft wrap after "b".
	st.Search = SearchState{Active: true, Current: -1, Matches: []SearchMatch{{Start: 3, End: 7}}, MatchStyle: hit}
	rows := st.WrappedRows(4, base)
	if rows[0][3].St != hit || rows[0][2].St != base {
		t.Fatalf("row 0 styles wrong: %+v", rows[0])
	}
	if rows[1][0].St != hit {
		t.Fatal("indent must take the style of the row's first text cell")
	}
	if st.HighlightedCells[3].St != base {
		t.Fatal("overlay must not touch stored cells")
	}
}

func TestSourceLineToScrollOffsetIsFirstRowOfLine(t *testing.T) {
	st := internalState("1 aaaaaa\n2 b\n3 c\n", 2)
	base := tcell.StyleDefault
	if got := st.SourceLineToScrollOffset(1, 4, base); got != 3 {
		t.Fatalf("line 1 row = %d, want 3 (line 0 takes rows 0-2)", got)
	}
	if got := st.SourceLineToScrollOffset(99, 4, base); got != 4 {
		t.Fatalf("beyond content = %d, want last row 4", got)
	}
}

func TestReflowsAtDraw(t *testing.T) {
	chroma := State{Phase: PhaseDone, Source: SourceInternalHighlighted, ChromaStyle: "monokai"}
	if !chroma.ReflowsAtDraw() {
		t.Fatal("finished Chroma body should reflow at draw")
	}
	for name, mutate := range map[string]func(*State){
		"markdown": func(s *State) { s.IsMarkdown = true },
		"archive":  func(s *State) { s.ChromaStyle = "" },
		"external": func(s *State) { s.Source = SourceExternalANSI },
		"running":  func(s *State) { s.Phase = PhaseRunning },
	} {
		st := chroma
		mutate(&st)
		if st.ReflowsAtDraw() {
			t.Fatalf("%s body must not reflow at draw", name)
		}
	}
}

// BenchmarkWrapLayoutBuild is the reflow cost of a body at DefaultMaxPreviewBytes.
func BenchmarkWrapLayoutBuild(b *testing.B) {
	line := "\tlantern := harbor(meadow, copper) // " + strings.Repeat("velvet ", 12) + "\n"
	text := strings.Repeat(line, 512*1024/len(line))
	st := internalState(text, 0)
	b.ReportAllocs()
	for b.Loop() {
		st.layout = nil
		st.EnsureWrapLayout(120, tcell.StyleDefault)
	}
}
