package uiscrollbar

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDraw_thumbOnBorder(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	const (
		x       = 19
		listTop = 2
		visible = 5
	)
	m, ok := ComputeMetrics(50, visible, 10)
	if !ok {
		t.Fatal("expected metrics")
	}
	styles := theme.Default()
	frame := styles.PanelActiveFrame
	Draw(DrawParams{
		Screen:     screen,
		X:          x,
		ListTopY:   listTop,
		Visible:    visible,
		Metrics:    m,
		Style:      StyleThumb,
		Active:     true,
		FrameStyle: frame,
		Theme:      styles,
	})

	thumbRow := listTop + m.ThumbStart + m.ThumbSize/2
	got, _, _ := screen.Get(x, thumbRow)
	wantThumb := styles.SymbolScrollbarThumb()
	gotR, _ := utf8.DecodeRuneInString(got)
	if gotR != wantThumb {
		t.Fatalf("thumb cell = %q, want %q", got, string(wantThumb))
	}
	trackRow := listTop
	if trackRow == thumbRow && visible > 1 {
		trackRow = listTop + 1
	}
	got, _, _ = screen.Get(x, trackRow)
	gotR, _ = utf8.DecodeRuneInString(got)
	if gotR != '│' {
		t.Fatalf("track cell at %d = %q, want │", trackRow, got)
	}
}

func TestDraw_barThumbSpan(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	const (
		x       = 14
		listTop = 2
		visible = 8
	)
	m, ok := ComputeMetrics(80, visible, 20)
	if !ok {
		t.Fatal("expected metrics")
	}
	styles := theme.Default()
	Draw(DrawParams{
		Screen:     screen,
		X:          x,
		ListTopY:   listTop,
		Visible:    visible,
		Metrics:    m,
		Style:      StyleBar,
		Active:     true,
		FrameStyle: styles.PanelActiveFrame,
		Theme:      styles,
	})

	col := tcelltest.TextAt(screen, x, listTop, 1)
	for row := 0; row < visible; row++ {
		cell, _, _ := screen.Get(x, listTop+row)
		r, _ := utf8.DecodeRuneInString(cell)
		if r != '█' && r != '░' {
			t.Fatalf("row %d = %q, want █ or ░ (col %q)", row, cell, col)
		}
	}
}

func TestDraw_noneSkipsPaint(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	const x = 5
	screen.SetContent(x, 2, 'X', nil, tcell.StyleDefault)
	m, _ := ComputeMetrics(30, 5, 3)
	Draw(DrawParams{
		Screen:   screen,
		X:        x,
		ListTopY: 2,
		Visible:  5,
		Metrics:  m,
		Style:    StyleNone,
		Theme:    theme.Default(),
	})
	got, _, _ := screen.Get(x, 2)
	if got != "X" {
		t.Fatalf("cell = %q, want unchanged X", got)
	}
}
