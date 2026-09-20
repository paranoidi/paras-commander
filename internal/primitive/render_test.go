package primitive

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
)

func TestTruncateRightUsesOverflowMarker(t *testing.T) {
	got := TruncateRight("abcdef", 4)
	want := "abc" + string(Ellipsis)
	if got != want {
		t.Fatalf("TruncateRight() = %q, want %q", got, want)
	}
}

func TestTruncateMiddlePreservesBothEnds(t *testing.T) {
	got := TruncateMiddle("abcdef", 5)
	want := "ab" + string(Ellipsis) + "ef"
	if got != want {
		t.Fatalf("TruncateMiddle() = %q, want %q", got, want)
	}
}

func TestTextPadsRemainingCells(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(10, 2)

	Text(screen, 0, 0, 5, "go", tcell.StyleDefault)

	if got := tcelltest.TextAt(screen, 0, 0, 5); got != "go   " {
		t.Fatalf("text = %q, want padded content", got)
	}
}

func TestTextUsesTerminalCellWidth(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	Text(screen, 0, 0, 4, "中中", tcell.StyleDefault)
	if got := tcelltest.TextAt(screen, 0, 0, 4); got != "中中" {
		t.Fatalf("CJK text = %q, want 中中 across 4 cells", got)
	}

	Text(screen, 0, 1, 3, "中中", tcell.StyleDefault)
	if got := tcelltest.TextAt(screen, 0, 1, 3); got != "中…" {
		t.Fatalf("CJK overflow = %q, want 中…", got)
	}

	Text(screen, 0, 2, 4, "😀A", tcell.StyleDefault)
	got := tcelltest.TextAt(screen, 0, 2, 4)
	if !strings.Contains(got, "😀") || !strings.Contains(got, "A") {
		t.Fatalf("emoji text = %q, want 😀 and A", got)
	}
}

func TestStyledTextHighlightsByRuneIndex(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(8, 2)

	hi := tcell.StyleDefault.Bold(true)
	StyledText(screen, 0, 0, 6, "中AB", tcell.StyleDefault, []Span{{Start: 0, End: 1, Style: hi}})
	got := tcelltest.TextAt(screen, 0, 0, 6)
	if !strings.Contains(got, "中") || !strings.Contains(got, "A") || !strings.Contains(got, "B") {
		t.Fatalf("styled text = %q", got)
	}
	main, st, _ := screen.Get(0, 0)
	if main != "中" {
		t.Fatalf("cell 0 = %q, want 中", main)
	}
	_, _, attrs := st.Decompose()
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("first CJK rune span was not applied to its cells")
	}
}

func TestTextCombiningMarkStaysOnOneCell(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(6, 2)

	Text(screen, 0, 0, 4, "e\u0301X", tcell.StyleDefault)
	main, _, width := screen.Get(0, 0)
	if !strings.Contains(main, "e") || !strings.Contains(main, "\u0301") {
		t.Fatalf("cell 0 = %q, want e plus U+0301", main)
	}
	if width != 1 {
		t.Fatalf("combining cluster width = %d, want 1", width)
	}
}

func TestBoxDrawsCorners(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(5, 4)

	Box(screen, Rect{X: 1, Y: 1, Width: 3, Height: 2}, tcell.StyleDefault, SharpBorder)

	tests := []struct {
		name string
		x    int
		y    int
		want rune
	}{
		{name: "top left", x: 1, y: 1, want: '┌'},
		{name: "top right", x: 3, y: 1, want: '┐'},
		{name: "bottom left", x: 1, y: 2, want: '└'},
		{name: "bottom right", x: 3, y: 2, want: '┘'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str, _, _ := screen.Get(tt.x, tt.y)
			var got rune
			if str != "" {
				got, _ = utf8.DecodeRuneInString(str)
			}
			if got != tt.want {
				t.Fatalf("corner = %q, want %q", got, tt.want)
			}
		})
	}
}
