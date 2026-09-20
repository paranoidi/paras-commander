package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestExtractDialogRendersSkippedItemWarning(t *testing.T) {
	t.Parallel()
	const w, h = 80, 24
	layout := Layout{Width: w, Height: h}
	styles := theme.Default()

	// Mixed selection: two generated archives plus two non-archives (the warning
	// OpenExtractDialog already computes as FileDialogState.Message).
	withMsg := FileDialogState{
		Open:       true,
		DialogType: FileDialogExtract,
		Fields: []FileDialogField{{
			Label: "Destination",
			Value: "/home/user/meadow",
		}},
		Message: "2 non-archive item(s) will be skipped.",
		ExtractSources: []string{
			"/tmp/harbor.tar.gz",
			"/tmp/willow.zip",
		},
	}
	withoutMsg := withMsg
	withoutMsg.Message = ""

	rectWith, ok := FileDialogRect(layout, withMsg, 0)
	if !ok {
		t.Fatal("extract dialog with warning not drawable")
	}
	rectWithout, ok := FileDialogRect(layout, withoutMsg, 0)
	if !ok {
		t.Fatal("extract dialog without warning not drawable")
	}
	if rectWith.Height <= rectWithout.Height {
		t.Fatalf("height with message = %d, want greater than %d", rectWith.Height, rectWithout.Height)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)
	DrawFileDialog(screen, layout, withMsg, DialogRenderContext{Styles: styles}, nil)

	rows := make([]string, h)
	for y := 0; y < h; y++ {
		rows[y] = tcelltest.TextAt(screen, 0, y, w)
	}
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, "2 non-archive item(s) will be skipped.") {
		t.Fatalf("skipped-item warning not drawn; rows:\n%s", joined)
	}

	buttonY := -1
	for y, row := range rows {
		if strings.Contains(row, "[ OK ]") {
			buttonY = y
			break
		}
	}
	if buttonY < 3 {
		t.Fatalf("OK button not found; rows:\n%s", joined)
	}
	// Separator sits on buttonY-1; the mandatory blank row is immediately above it.
	if !strings.Contains(rows[buttonY-1], "─") {
		t.Fatalf("row above buttons = %q, want separator", strings.TrimSpace(rows[buttonY-1]))
	}
	blank := strings.Trim(rows[buttonY-2], " │")
	if blank != "" {
		t.Fatalf("blank row above button separator = %q, want empty dialog surface", strings.TrimSpace(rows[buttonY-2]))
	}
}
