package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawMassRenameDialogShowsRegexpCompileHint(t *testing.T) {
	_, err := ops.MassRenameCompileRegex(`\`)
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
	DrawFileDialog(screen, layout, state, styles, false, 0, nil)

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
