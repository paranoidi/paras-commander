package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func TestDrawFindDialogSelectionSizeOnSeparator(t *testing.T) {
	if !FindDialogSelectionSizeEnabled {
		t.Skip("find dialog selection size temporarily disabled")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	layout := Layout{Width: 80, Height: 24}
	state := FindDialogState{Open: true}
	styles := theme.Default()
	selectionLabel := " 2 items (1 KiB) "

	DrawFindDialog(screen, layout, state, styles, false, 0, nil, selectionLabel)

	width, height, listH, ok := FindDialogMetrics(layout, false)
	if !ok {
		t.Fatal("FindDialogMetrics: want ok")
	}
	rect := draw.CenteredDialogRect(layout, width, height)
	checkboxRows := 1
	sepAfterCheckbox := rect.Y + 5 + checkboxRows
	listTop := sepAfterCheckbox + 1
	buttonY := rect.Y + height - 2
	sepY := listTop + listH
	if sepY >= buttonY {
		sepY = buttonY - 1
	}
	innerLeft := rect.X + 1

	wantStyle := styles.DialogIndicatorSelectionSizeStyle()
	_, wantBG, _ := wantStyle.Decompose()
	found := false
	for x := innerLeft; x < rect.X+rect.Width-1; x++ {
		ch, st, _ := screen.Get(x, sepY)
		if ch == "2" {
			found = true
			_, bg, _ := st.Decompose()
			if bg != wantBG {
				t.Fatalf("label bg = %v, want dialog.surface bg %v", bg, wantBG)
			}
			if st != wantStyle {
				t.Fatalf("label style = %v, want %v", st, wantStyle)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected selection size label on separator above buttons")
	}
}
