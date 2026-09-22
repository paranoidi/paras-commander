package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func newSortDialogTestScreen(t *testing.T) (tcell.SimulationScreen, Layout) {
	t.Helper()
	const w, h = 80, 24
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(w, h)
	return screen, Layout{Width: w, Height: h}
}

// rowText reads the full dialog row at y as a string, for substring checks.
func rowText(screen tcell.SimulationScreen, rect draw.Rect, y int) string {
	var sb strings.Builder
	for x := rect.X; x < rect.X+rect.Width; x++ {
		ch, _, _ := screen.Get(x, y)
		sb.WriteString(ch)
	}
	return sb.String()
}

// TestSortDialogMetaRadios_heightUnchanged confirms up to 4 active meta columns are drawn in a
// second radio column on the same rows as the built-in radios, without growing the dialog.
func TestSortDialogMetaRadios_heightUnchanged(t *testing.T) {
	screen, layout := newSortDialogTestScreen(t)

	DrawSortDialog(screen, layout, SortDialogState{Open: true, SortMode: panel.SortName}, asciiDialogStyles())
	rectNoMeta, ok := draw.ClampCenteredDialogRect(layout, 56, 11, 30, 11)
	if !ok {
		t.Fatal("sort dialog rect not drawable")
	}

	screen.Clear()
	st := SortDialogState{
		Open:     true,
		SortMode: panel.SortMeta,
		MetaRadios: []SortDialogMetaRadio{
			{Title: "duration", Name: "duration"},
			{Title: "bitrate", Name: "bitrate"},
			{Title: "codec", Name: "codec"},
			{Title: "resolution", Name: "resolution"},
		},
		MetaColumn: "codec",
	}
	DrawSortDialog(screen, layout, st, asciiDialogStyles())
	rectMeta, ok := draw.ClampCenteredDialogRect(layout, 56, 11, 30, 11)
	if !ok {
		t.Fatal("sort dialog rect not drawable with 4 meta columns")
	}
	if rectMeta != rectNoMeta {
		t.Fatalf("rect changed with 4 meta columns: got %+v, want %+v", rectMeta, rectNoMeta)
	}

	for i, radio := range st.MetaRadios {
		y := rectMeta.Y + 1 + i
		row := rowText(screen, rectMeta, y)
		if !strings.Contains(row, radio.Title) {
			t.Fatalf("row %d (%q) missing meta title %q", y, row, radio.Title)
		}
	}
	// The selected meta radio (codec, row index 2) shows the selected marker.
	selectedRow := rowText(screen, rectMeta, rectMeta.Y+1+2)
	if !strings.Contains(selectedRow, "(*)") {
		t.Fatalf("codec row = %q, want a selected radio marker (*) ", selectedRow)
	}
}

// TestSortDialogMetaRadios_cappedAtFour confirms a 5th active meta column never appears — the
// dialog always caps at 4 meta radios (one per built-in radio row) so it never grows taller.
func TestSortDialogMetaRadios_cappedAtFour(t *testing.T) {
	screen, layout := newSortDialogTestScreen(t)

	st := SortDialogState{
		Open:     true,
		SortMode: panel.SortMeta,
		MetaRadios: []SortDialogMetaRadio{
			{Title: "duration", Name: "duration"},
			{Title: "bitrate", Name: "bitrate"},
			{Title: "codec", Name: "codec"},
			{Title: "resolution", Name: "resolution"},
			{Title: "overflow-column", Name: "overflow-column"},
		},
	}
	if got, want := st.MetaCount(), len(panel.SortDialogRadios()); got != want {
		t.Fatalf("MetaCount() = %d, want %d", got, want)
	}

	DrawSortDialog(screen, layout, st, asciiDialogStyles())
	rect, ok := draw.ClampCenteredDialogRect(layout, 56, 11, 30, 11)
	if !ok {
		t.Fatal("sort dialog rect not drawable")
	}

	for y := rect.Y; y < rect.Y+rect.Height; y++ {
		row := rowText(screen, rect, y)
		if strings.Contains(row, "overflow-column") {
			t.Fatalf("row %d (%q) drew the 5th (capped-out) meta column", y, row)
		}
	}
}
