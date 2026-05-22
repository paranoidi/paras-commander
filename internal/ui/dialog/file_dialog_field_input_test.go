package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawInputFieldScrollsHorizontallyForLongValue(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	styles := theme.Default()
	value := "/home/user/very/long/path/to/something"
	const width = 20
	field := FileDialogField{Value: value, Cursor: len([]rune(value))}

	drawInputField(screen, 1, 1, width, field, true, styles)

	got := tcelltest.TextAt(screen, 1, 1, width)
	if strings.Contains(got, "~") {
		t.Fatalf("did not expect ~ truncation marker, got %q", got)
	}
	tail := []rune(value)[len([]rune(value))-(width-2):]
	if !strings.Contains(got, string(tail)) {
		t.Fatalf("expected tail %q in visible row %q", string(tail), got)
	}
	if !strings.Contains(got, "◀") {
		t.Fatalf("expected ◀ overflow marker in %q", got)
	}
}

func TestDrawPathInputRowInvalidGhostAndGlyphAvoidErrorStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	styles := theme.Default()

	value := "/tmp/x"
	suffix := "YZ"
	cursor := len([]rune(value))
	const width = 22
	textW := width - 2
	field := FileDialogField{
		Value:            value,
		Cursor:           cursor,
		CompletionSuffix: suffix,
		PathPicker:       true,
	}

	drawPathInputRow(screen, 1, 1, width, field, true, false, true, styles)

	wantGhost := styles.DialogInputActivePlaceholder
	found := false
	for col := 1; col < 1+textW; col++ {
		ch, gotSt, _ := screen.Get(col, 1)
		if ch != "Z" {
			continue
		}
		found = true
		gotFG, gotBG, gotAttr := gotSt.Decompose()
		wantFG, wantBG, wantAttr := wantGhost.Decompose()
		if gotFG != wantFG || gotBG != wantBG || gotAttr != wantAttr {
			t.Fatalf("ghost style mismatch: got fg=%v bg=%v attr=%v want fg=%v bg=%v attr=%v",
				gotFG, gotBG, gotAttr, wantFG, wantBG, wantAttr)
		}
		if gotSt == styles.DialogInputActiveError {
			t.Fatal("ghost completion must not use error style")
		}
		break
	}
	if !found {
		t.Fatalf("ghost char %q not found in %q", "Z", tcelltest.TextAt(screen, 1, 1, textW))
	}

	wantGlyph := styles.DialogInputBaseStyle(true, false)
	_, glyphSt, _ := screen.Get(1+textW, 1)
	if glyphSt == styles.DialogInputActiveError {
		t.Fatal("path-picker glyph must not use error style")
	}
	gotFG, gotBG, gotAttr := glyphSt.Decompose()
	wantFG, wantBG, wantAttr := wantGlyph.Decompose()
	if gotFG != wantFG || gotBG != wantBG || gotAttr != wantAttr {
		t.Fatalf("glyph style fg=%v bg=%v attr=%v want fg=%v bg=%v attr=%v",
			gotFG, gotBG, gotAttr, wantFG, wantBG, wantAttr)
	}
}

func TestDrawPathInputRowScrollsHorizontallyForLongValue(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	styles := theme.Default()
	value := "/home/user/very/long/path/to/something"
	const width = 22 // textW = 20
	field := FileDialogField{
		Value:      value,
		Cursor:     len([]rune(value)),
		Scroll:     0,
		PathPicker: true,
	}

	drawPathInputRow(screen, 1, 1, width, field, true, false, false, styles)

	got := tcelltest.TextAt(screen, 1, 1, width-2)
	if strings.Contains(got, "~") {
		t.Fatalf("did not expect ~ truncation marker, got %q", got)
	}
	if !strings.Contains(got, "◀") {
		t.Fatalf("expected ◀ overflow marker in %q", got)
	}
}
