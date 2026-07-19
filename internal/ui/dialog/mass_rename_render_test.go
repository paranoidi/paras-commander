package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawMassRenameDialogLastItemVisible(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)

	before := []string{"apple.txt", "banana.txt", "cherry.txt", "delta.txt", "echo.txt"}
	after := make([]string, len(before))
	copy(after, before)

	state := FileDialogState{
		Open:       true,
		DialogType: FileDialogMassRename,
		Fields: []FileDialogField{
			{Label: "Find", Value: ""},
			{Label: "Replace", Value: ""},
		},
		MassRenameMode:          MassRenameModeUISimple,
		MassRenamePreviewBefore: before,
		MassRenamePreviewAfter:  after,
		MassRenamePreviewScroll: 0,
	}
	layout := Layout{Width: 80, Height: 40}
	styles := theme.Default()
	DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil)

	var dump strings.Builder
	for y := 0; y < 40; y++ {
		dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
		dump.WriteByte('\n')
	}
	if !strings.Contains(dump.String(), "echo.txt") {
		t.Fatalf("last preview item (echo.txt) not visible on screen:\n%s", dump.String())
	}
}

func TestDrawMassRenameDialogShowsRegexpCompileHint(t *testing.T) {
	_, err := ops.MassRenameCompileRegex(`\`, false)
	if err == nil {
		t.Fatal("expected compile error")
	}
	hint := ops.MassRenameRegexCompileUserMessage(err)
	if hint == "" {
		t.Fatal("expected non-empty hint")
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	state := FileDialogState{
		Open:       true,
		DialogType: FileDialogMassRename,
		Fields: []FileDialogField{
			{Label: "Pattern", Value: `\`, Cursor: 1, InputInvalid: true},
			{Label: "Replacement", Value: "", Cursor: 0},
		},
		FocusedField:                 2,
		MassRenameMode:               MassRenameModeUIRegex,
		MassRenamePatternCompileHint: hint,
		MassRenamePreviewBefore:      []string{"a.txt"},
		MassRenamePreviewAfter:       []string{"a.txt"},
	}
	layout := Layout{Width: 80, Height: 24}
	styles := theme.Default()
	DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil)

	var found string
	for y := 0; y < 24; y++ {
		line := tcelltest.TextAt(screen, 2, y, 76)
		if strings.Contains(line, "backslash") {
			found = strings.TrimSpace(line)
			break
		}
	}
	if found == "" {
		var dump strings.Builder
		for y := 6; y <= 14; y++ {
			dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
			dump.WriteByte('\n')
		}
		t.Fatalf("regexp hint not painted on screen; hint=%q\nscreen dump:\n%s", hint, dump.String())
	}
}
