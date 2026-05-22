package draw

import (
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
	PaintScrollingInputContent(screen, 2, 2, 16, value, suffix, cursor, 0, true, true, true, th)

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
