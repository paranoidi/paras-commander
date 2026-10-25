package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

const buttonBlankRowTermW, buttonBlankRowTermH = 80, 24

func buttonBlankRowScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(buttonBlankRowTermW, buttonBlankRowTermH)
	return screen
}

func dialogScreenRows(screen tcell.SimulationScreen) []string {
	rows := make([]string, buttonBlankRowTermH)
	for y := 0; y < buttonBlankRowTermH; y++ {
		rows[y] = tcelltest.TextAt(screen, 0, y, buttonBlankRowTermW)
	}
	return rows
}

func dialogButtonRowY(rows []string) int {
	for y, row := range rows {
		if strings.Contains(row, "[ OK ]") ||
			strings.Contains(row, "[ Cancel ]") ||
			strings.Contains(row, "[ Replace ]") ||
			strings.Contains(row, "[ Yes ]") {
			return y
		}
	}
	return -1
}

// assertButtonsUnderSeparator requires the separator on the row immediately above the first
// button row: dlg dialogs have no blank row there.
func assertButtonsUnderSeparator(t *testing.T, rows []string) {
	t.Helper()
	buttonY := dialogButtonRowY(rows)
	if buttonY < 1 || !strings.Contains(rows[buttonY-1], "├") {
		t.Fatalf("row above buttons = %q, want the separator (dlg dialogs have no blank row there)", rows[buttonY-1])
	}
}

func TestFileDialogRenameButtonsSitUnderSeparator(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawFileDialog(screen, layout, FileDialogState{
		Open:       true,
		DialogType: FileDialogRename,
		Fields:     []FileDialogField{{Label: "New name", Value: "harborlantern.txt"}},
	}, DialogRenderContext{Styles: theme.Default()}, nil)
	assertButtonsUnderSeparator(t, dialogScreenRows(screen))
}

func TestTransferCopyButtonsSitUnderSeparator(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawTransferDialog(screen, layout, TransferDialogState{
		Open:        true,
		Kind:        TransferKindCopy,
		Phase:       TransferPhaseDestination,
		Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
	}, DialogRenderContext{Styles: theme.Default()}, nil)
	assertButtonsUnderSeparator(t, dialogScreenRows(screen))
}

func TestTransferMoveButtonsSitUnderSeparator(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawTransferDialog(screen, layout, TransferDialogState{
		Open:        true,
		Kind:        TransferKindMove,
		Phase:       TransferPhaseDestination,
		Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
	}, DialogRenderContext{Styles: theme.Default()}, nil)
	assertButtonsUnderSeparator(t, dialogScreenRows(screen))
}

func TestTransferSelfCopyButtonsSitUnderSeparator(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawTransferDialog(screen, layout, TransferDialogState{
		Open:            true,
		Kind:            TransferKindCopy,
		Phase:           TransferPhaseSelfCopyRename,
		SelfCopyNewName: FileDialogField{Value: "badger copy.txt"},
	}, DialogRenderContext{Styles: theme.Default()}, nil)
	assertButtonsUnderSeparator(t, dialogScreenRows(screen))
}

func TestTransferMultiLocationButtonsSitUnderSeparator(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawTransferDialog(screen, layout, TransferDialogState{
		Open:        true,
		Kind:        TransferKindCopy,
		Phase:       TransferPhaseDestination,
		Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
		CommonRoot:  "/home/user/thicket",
		Entries: []DeleteListEntry{
			{Name: "badger.txt", Path: "/home/user/thicket/badger.txt", Type: localfs.EntryFile},
			{Name: "otter.txt", Path: "/home/user/thicket/otter.txt", Type: localfs.EntryFile},
		},
	}, DialogRenderContext{Styles: theme.Default()}, nil)
	assertButtonsUnderSeparator(t, dialogScreenRows(screen))
}

func TestFlattenButtonsSitUnderSeparator(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawFlattenDialog(screen, layout, FlattenDialogState{
		Open:        true,
		Destination: FileDialogField{Value: "/home/user/meadow", PathPicker: true},
	}, theme.Default(), uiscrollbar.StyleThumb)
	rows := dialogScreenRows(screen)
	buttonY := dialogButtonRowY(rows)
	if buttonY < 1 || !strings.Contains(rows[buttonY-1], "├") {
		t.Fatalf("row above buttons = %q, want the separator (dlg dialogs have no blank row there)", rows[buttonY-1])
	}
}

func TestPathPickerButtonsSitUnderSeparator(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawPathPickerDialog(screen, layout, PathPickerState{
		Open:   true,
		Title:  "Bookmarks",
		Items:  []PathPickerItem{{Source: "fzf-marks", Name: "harbor", Path: "/tmp/harbor"}},
		Ranked: []int{0},
	}, theme.Default(), uiscrollbar.StyleThumb, nil)
	rows := dialogScreenRows(screen)
	buttonY := dialogButtonRowY(rows)
	if buttonY < 1 || !strings.Contains(rows[buttonY-1], "├") {
		t.Fatalf("row above buttons = %q, want the separator (dlg dialogs have no blank row there)", rows[buttonY-1])
	}
}

func TestStashRestoreButtonsSitUnderSeparatorAndNoHelpFooter(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawStashRestoreDialog(screen, layout, StashRestoreDialogState{}, theme.Default())
	rows := dialogScreenRows(screen)
	buttonY := dialogButtonRowY(rows)
	if buttonY < 1 || !strings.Contains(rows[buttonY-1], "├") {
		t.Fatalf("row above buttons = %q, want the separator (dlg dialogs have no blank row there)", rows[buttonY-1])
	}
	if !strings.Contains(rows[buttonY], "[ Replace ]") {
		t.Fatalf("button row = %q, want Replace (help text must not overwrite buttons)", strings.TrimSpace(rows[buttonY]))
	}
	if !strings.Contains(rows[buttonY], "[ Cancel ]") {
		t.Fatalf("button row = %q, want Cancel", strings.TrimSpace(rows[buttonY]))
	}
	joined := strings.Join(rows, "\n")
	if strings.Contains(joined, "Left/Right") || strings.Contains(joined, "Enter confirm") {
		t.Fatalf("stash restore still paints a navigation help footer; rows:\n%s", joined)
	}
}
