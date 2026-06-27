package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// TestRenderAddBookmarkDialogShowsTitlePathAndName verifies the new
// Add bookmark file dialog draws its centered title, the read-only
// "Path:" info row populated from dialog.FileDialogState.Message, the editable
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
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		FileDialog: dialog.FileDialogState{
			Open:       true,
			DialogType: dialog.FileDialogAddBookmark,
			Fields: []dialog.FileDialogField{
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

func findRow(screen tcell.SimulationScreen, width, height int, needle string) bool {
	for y := 0; y < height; y++ {
		if strings.Contains(tcelltest.TextAt(screen, 0, y, width), needle) {
			return true
		}
	}
	return false
}

// TestRenderMkdirDialogWithoutSelectionHidesActionRadios verifies the F7 dialog
// renders only the directory-name field plus OK/Cancel when MkdirShowActions is off.
func TestRenderMkdirDialogWithoutSelectionHidesActionRadios(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	const width = 80

	styles := theme.Default()
	model := Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		FileDialog: dialog.FileDialogState{
			Open:       true,
			DialogType: dialog.FileDialogMkdir,
			Fields:     []dialog.FileDialogField{{Label: "Directory name", Value: "x"}},
		},
	}

	Render(screen, model, styles)

	if !findRow(screen, width, 24, "Create directory") {
		t.Fatal("expected 'Create directory' title")
	}
	if findRow(screen, width, 24, "and copy selected") {
		t.Fatal("Create-and-copy-selected radio must not render when MkdirShowActions is false")
	}
	if findRow(screen, width, 24, "and move selected") {
		t.Fatal("Create-and-move-selected radio must not render when MkdirShowActions is false")
	}
}

// TestRenderMkdirDialogWithSelectionShowsActionRadios verifies all three radio
// labels are drawn when MkdirShowActions is enabled.
func TestRenderMkdirDialogWithSelectionShowsActionRadios(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	const width = 80

	styles := theme.Default()
	model := Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		FileDialog: dialog.FileDialogState{
			Open:             true,
			DialogType:       dialog.FileDialogMkdir,
			Fields:           []dialog.FileDialogField{{Label: "Directory name", Value: "x"}},
			MkdirShowActions: true,
			MkdirAction:      dialog.MkdirActionCreate,
		},
	}

	Render(screen, model, styles)

	for _, label := range []string{"Create", "and copy selected", "and move selected"} {
		if !findRow(screen, width, 24, label) {
			t.Fatalf("expected radio label %q on screen", label)
		}
	}
}
