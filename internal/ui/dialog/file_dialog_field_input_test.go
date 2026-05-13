package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
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

	got := textAt(screen, 1, 1, width)
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
	field := FileDialogField{Value: value, Cursor: len([]rune(value)), PathPicker: true}

	drawPathInputRow(screen, 1, 1, width, field, true, false, false, styles)

	got := textAt(screen, 1, 1, width-2)
	if strings.Contains(got, "~") {
		t.Fatalf("did not expect ~ truncation marker, got %q", got)
	}
	if !strings.Contains(got, "◀") {
		t.Fatalf("expected ◀ overflow marker in %q", got)
	}
}
