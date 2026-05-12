package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// TestRenderAddBookmarkDialogShowsTitlePathAndName verifies the new
// Add bookmark file dialog draws its centered title, the read-only
// "Path:" info row populated from FileDialogState.Message, the editable
// "Name:" label, and the prefilled value in the input row.
func TestRenderAddBookmarkDialogShowsTitlePathAndName(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	const width = 80

	styles := theme.Default()
	model := Model{
		Left:        panel.State{Path: "/tmp"},
		Right:       panel.State{Path: "/var"},
		ActivePanel: LeftPanel,
		FileDialog: FileDialogState{
			Open:       true,
			DialogType: FileDialogAddBookmark,
			Fields: []FileDialogField{
				{Label: "Name", Value: "projects", Prefill: "projects", Cursor: 8, PrefillPending: true},
			},
			Message: "/home/user/projects",
		},
	}

	Render(screen, model, styles)

	if !findRow(screen, width, 24, "Add bookmark") {
		t.Fatal("expected dialog title 'Add bookmark' on the screen")
	}
	if !findRow(screen, width, 24, "Path:") {
		t.Fatal("expected 'Path:' label row on the screen")
	}
	if !findRow(screen, width, 24, "/home/user/projects") {
		t.Fatal("expected the read-only path row on the screen")
	}
	if !findRow(screen, width, 24, "Name:") {
		t.Fatal("expected 'Name:' label row on the screen")
	}
	if !findRow(screen, width, 24, "projects") {
		t.Fatal("expected the prefilled name to render in the input row")
	}
	if !findRow(screen, width, 24, "[ O Cancel ]") && !findRow(screen, width, 24, "OK") {
		t.Fatal("expected OK button row")
	}
}

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

func findRow(screen tcell.SimulationScreen, width, height int, needle string) bool {
	for y := 0; y < height; y++ {
		if strings.Contains(textAt(screen, 0, y, width), needle) {
			return true
		}
	}
	return false
}
