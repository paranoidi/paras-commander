package draw

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPaintScrollingInputGhostNotErrorWhenInvalid(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 10)

	th := theme.Default()
	value := "/tmp/x"
	suffix := "YZ"
	cursor := len([]rune(value))
	PaintScrollingInputContent(screen, 2, 2, 16, value, suffix, cursor, 0, true, true, true, false, "", th)

	col := 2 + cursor + len([]rune(suffix)) - 1
	_, gotSt, _ := screen.Get(col, 2)
	want := th.DialogInputActivePlaceholder
	gotFG, gotBG, gotAttr := gotSt.Decompose()
	wantFG, wantBG, wantAttr := want.Decompose()
	if gotFG != wantFG || gotBG != wantBG || gotAttr != wantAttr {
		t.Fatalf("ghost style fg=%v bg=%v attr=%v want placeholder fg=%v bg=%v attr=%v",
			gotFG, gotBG, gotAttr, wantFG, wantBG, wantAttr)
	}
	errFG, _, _ := th.DialogInputActiveError.Decompose()
	if gotFG == errFG && gotSt == th.DialogInputActiveError {
		t.Fatalf("ghost cell uses error style")
	}
}

// screenRow reads width cells at (x, y), skipping the continuation cell of 2-cell glyphs.
func screenRow(screen tcell.Screen, x, y, width int) (string, []tcell.Style) {
	var b strings.Builder
	var styles []tcell.Style
	for col := 0; col < width; {
		str, st, w := screen.Get(x+col, y)
		b.WriteString(str)
		styles = append(styles, st)
		col += max(1, w)
	}
	return b.String(), styles
}

func TestPaintScrollingInputWideGlyphs(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 4)
	th := theme.Default()
	value := "森の黒魔術と大量の画質" // 11 runes, 22 cells

	PaintScrollingInputContent(screen, 0, 0, 12, value, "", 0, 0, true, false, true, false, "", th)
	if got, _ := screenRow(screen, 0, 0, 12); got != "森の黒魔術 ▶" {
		t.Fatalf("cursor at start: row = %q, want %q", got, "森の黒魔術 ▶")
	}

	cursor := len([]rune(value))
	PaintScrollingInputContent(screen, 0, 1, 12, value, "", cursor, 0, true, false, true, false, "", th)
	got, sts := screenRow(screen, 0, 1, 12)
	if got != "◀大量の画質 " {
		t.Fatalf("cursor at end: row = %q, want %q", got, "◀大量の画質 ")
	}
	if _, _, attrs := sts[6].Decompose(); attrs&tcell.AttrReverse == 0 {
		t.Fatalf("caret cell after last glyph is not reversed")
	}
}

// The caret keeps the input's own colors whatever text it sits next to: reversing the dimmed
// placeholder style would paint it in the dialog surface color.
func TestPaintScrollingInputCaretUsesInputStyle(t *testing.T) {
	th := theme.Default()
	value := "/meadow/lantern"
	end := len([]rune(value))
	cases := []struct {
		name          string
		suffix        string
		cursor        int
		invalid       bool
		asPlaceholder bool
		want          tcell.Style
	}{
		{name: "pending prefill", cursor: end, asPlaceholder: true, want: th.DialogInputActive.Reverse(true)},
		{name: "ghost completion", suffix: "-harbor", cursor: end, want: th.DialogInputActive.Reverse(true)},
		{name: "typed text", cursor: end, want: th.DialogInputActive.Reverse(true)},
		{name: "invalid ghost completion", suffix: "-harbor", cursor: end, invalid: true, want: th.DialogInputActiveError.Reverse(true)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			t.Cleanup(screen.Fini)
			screen.SetSize(40, 4)

			PaintScrollingInputContent(screen, 0, 0, 30, value, tc.suffix, tc.cursor, 0, true, tc.invalid, true, tc.asPlaceholder, "", th)

			if _, got, _ := screen.Get(tc.cursor, 0); got != tc.want {
				t.Fatalf("caret style = %v, want %v", got, tc.want)
			}
		})
	}
}
