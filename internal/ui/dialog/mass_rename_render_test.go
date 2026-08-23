package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// TestDrawMassRenameDialogShowsBangPrefixedNameAsPreviewRow guards against treating
// basenames that start with '!' as compute-error banners (they are legitimate names).
func TestDrawMassRenameDialogShowsBangPrefixedNameAsPreviewRow(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)

	state := FileDialogState{
		Open:       true,
		DialogType: FileDialogMassRename,
		Fields: []FileDialogField{
			{Label: "Find", Value: "!"},
			{Label: "Replace", Value: "X"},
		},
		MassRenameMode:          MassRenameModeUISimple,
		MassRenamePreviewBefore: []string{"!important", "normal"},
		MassRenamePreviewAfter:  []string{"Ximportant", "normal"},
	}
	layout := Layout{Width: 80, Height: 40}
	styles := theme.Default()
	DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil)

	var dump strings.Builder
	for y := 0; y < 40; y++ {
		dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
		dump.WriteByte('\n')
	}
	out := dump.String()
	if !strings.Contains(out, "!important") {
		t.Fatalf("bang-prefixed basename missing from preview:\n%s", out)
	}
	if !strings.Contains(out, "Ximportant") {
		t.Fatalf("renamed after column missing for bang-prefixed basename:\n%s", out)
	}
}

// TestDrawMassRenameDialogShowsComputeError verifies MassRenameComputeError paints as a
// full-width warning instead of being encoded into a Before row with a '!' prefix.
func TestDrawMassRenameDialogShowsComputeError(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)

	const errMsg = "no files to rename"
	state := FileDialogState{
		Open:                   true,
		DialogType:             FileDialogMassRename,
		Fields:                 []FileDialogField{{Label: "Find"}, {Label: "Replace"}},
		MassRenameMode:         MassRenameModeUISimple,
		MassRenameComputeError: errMsg,
	}
	layout := Layout{Width: 80, Height: 40}
	styles := theme.Default()
	DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil)

	var dump strings.Builder
	for y := 0; y < 40; y++ {
		dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
		dump.WriteByte('\n')
	}
	if !strings.Contains(dump.String(), errMsg) {
		t.Fatalf("compute error %q not visible:\n%s", errMsg, dump.String())
	}
}

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

// TestMassRenamePreviewViewportRowsMatchesRenderedRows guards against
// MassRenamePreviewViewportRows (used for PgUp/PgDn paging and the scrollbar) drifting from the
// row count drawMassRenameDialog actually paints — the two were independent hand-tuned formulas
// before and disagreed by a couple of rows, capping scroll short of the list's true end. Runs
// across every mode at a height that forces the dialog to clamp (the case that exposed the bug).
func TestMassRenamePreviewViewportRowsMatchesRenderedRows(t *testing.T) {
	const n = 30
	before := make([]string, n)
	after := make([]string, n)
	for i := range before {
		before[i] = fmt.Sprintf("file-%02d.txt", i)
		after[i] = before[i]
	}

	cases := []struct {
		name  string
		state FileDialogState
	}{
		{"Simple", FileDialogState{MassRenameMode: MassRenameModeUISimple, Fields: []FileDialogField{{Label: "Find"}, {Label: "Replace"}}}},
		{"Regex", FileDialogState{MassRenameMode: MassRenameModeUIRegex, Fields: []FileDialogField{{Label: "Pattern"}, {Label: "Replacement"}}}},
		{"ExternalEditor", FileDialogState{MassRenameMode: MassRenameModeUIExternalEditor}},
		{"Capitalize", FileDialogState{MassRenameMode: MassRenameModeUICapitalize}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			defer screen.Fini()
			screen.SetSize(80, 24) // short enough that massRenameDialogHeight clamps

			state := tc.state
			state.Open = true
			state.DialogType = FileDialogMassRename
			state.MassRenamePreviewBefore = before
			state.MassRenamePreviewAfter = after

			layout := Layout{Width: 80, Height: 24}
			vp := MassRenamePreviewViewportRows(layout.Height, state)
			state.MassRenamePreviewScroll = n - vp // scroll to the reported true last page

			styles := theme.Default()
			DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil)

			var dump strings.Builder
			for y := 0; y < 24; y++ {
				dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
				dump.WriteByte('\n')
			}
			last := before[n-1]
			if !strings.Contains(dump.String(), last) {
				t.Fatalf("%s: last preview item (%s) not visible when scrolled to MassRenamePreviewViewportRows' reported max (vp=%d) — it has drifted from what drawMassRenameDialog actually draws:\n%s", tc.name, last, vp, dump.String())
			}
		})
	}
}

func TestDrawMassRenameDialogPatternLabelShowsMatchCount(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)

	state := FileDialogState{
		Open:       true,
		DialogType: FileDialogMassRename,
		Fields: []FileDialogField{
			{Label: "Find", Value: "foo"},
			{Label: "Replace"},
		},
		MassRenameMode:       MassRenameModeUISimple,
		MassRenameMatchCount: 3,
	}
	layout := Layout{Width: 80, Height: 40}
	styles := theme.Default()
	DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil)

	var dump strings.Builder
	for y := 0; y < 40; y++ {
		dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
		dump.WriteByte('\n')
	}
	if !strings.Contains(dump.String(), "Find (3):") {
		t.Fatalf("label row does not show match count as \"Find (3):\":\n%s", dump.String())
	}
}

func TestDrawMassRenameDialogPatternLabelHidesCountWhenEmpty(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 40)

	state := FileDialogState{
		Open:       true,
		DialogType: FileDialogMassRename,
		Fields: []FileDialogField{
			{Label: "Find", Value: ""},
			{Label: "Replace"},
		},
		MassRenameMode:       MassRenameModeUISimple,
		MassRenameMatchCount: 0,
	}
	layout := Layout{Width: 80, Height: 40}
	styles := theme.Default()
	DrawFileDialog(screen, layout, state, DialogRenderContext{Styles: styles}, nil)

	var dump strings.Builder
	for y := 0; y < 40; y++ {
		dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
		dump.WriteByte('\n')
	}
	if strings.Contains(dump.String(), "Find (0):") {
		t.Fatalf("label row shows \"(0)\" for an empty pattern; want plain \"Find:\":\n%s", dump.String())
	}
	if !strings.Contains(dump.String(), "Find:") {
		t.Fatalf("label row does not show plain \"Find:\" for an empty pattern:\n%s", dump.String())
	}
}

// TestDrawMassRenameDialogShowsRegexpCompileHint verifies the regexp compile-error hint is
// painted on the same row as the Pattern label, right-aligned flush against the right margin
// (mirroring how the match count is right-aligned on the Find/Pattern row).
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

	var labelRow string
	var labelY int
	found := false
	for y := 0; y < 24; y++ {
		line := tcelltest.TextAt(screen, 0, y, 80)
		if strings.Contains(line, "Pattern (") {
			labelRow = line
			labelY = y
			found = true
			break
		}
	}
	if !found {
		var dump strings.Builder
		for y := 6; y <= 14; y++ {
			dump.WriteString(tcelltest.TextAt(screen, 0, y, 80))
			dump.WriteByte('\n')
		}
		t.Fatalf("Pattern label row not found on screen:\n%s", dump.String())
	}

	// Strip trailing screen background, the right border, and its 1-space margin, so what
	// remains is exactly the dialog's right-aligned content on this row.
	trimmed := strings.TrimRight(labelRow, " ")
	trimmed = strings.TrimSuffix(trimmed, "│")
	trimmed = strings.TrimRight(trimmed, " ")
	if !strings.HasSuffix(trimmed, hint) {
		t.Fatalf("row %d = %q, want it to end with %q (regexp hint right-aligned on the Pattern label row)", labelY, labelRow, hint)
	}
}
