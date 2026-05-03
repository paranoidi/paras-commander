package primitive

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

func TestTruncateRightUsesOverflowMarker(t *testing.T) {
	got := TruncateRight("abcdef", 4)
	if got != "abc~" {
		t.Fatalf("TruncateRight() = %q, want %q", got, "abc~")
	}
}

func TestTruncateMiddlePreservesBothEnds(t *testing.T) {
	got := TruncateMiddle("abcdef", 5)
	if got != "ab~ef" {
		t.Fatalf("TruncateMiddle() = %q, want %q", got, "ab~ef")
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

	if got := textAt(screen, 0, 0, 5); got != "go   " {
		t.Fatalf("text = %q, want padded content", got)
	}
}

func TestBoxDrawsCorners(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(5, 4)

	Box(screen, Rect{X: 1, Y: 1, Width: 3, Height: 2}, tcell.StyleDefault)

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

func textAt(screen tcell.SimulationScreen, x, y, width int) string {
	runes := make([]rune, 0, width)
	for col := 0; col < width; {
		str, _, cw := screen.Get(x+col, y)
		if cw < 1 {
			cw = 1
		}
		var r = ' '
		if str != "" {
			r, _ = utf8.DecodeRuneInString(str)
		}
		runes = append(runes, r)
		col += cw
	}
	return string(runes)
}
