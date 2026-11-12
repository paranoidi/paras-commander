package draw

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDialogButtonUsesThemeCaps(t *testing.T) {
	th := theme.Default()
	if got := DialogButtonWidth("OK"); got != 8 { // " [ OK ] "
		t.Fatalf("default width = %d, want 8", got)
	}
	th.Icons = map[string]string{theme.IconKeyDialogButtonStart: "▎ ", theme.IconKeyDialogButtonEnd: " ▕"}
	th.DialogButtonInactiveCaps.Start = tcell.StyleDefault.Foreground(tcell.ColorRed)
	SetButtonCaps(th)
	t.Cleanup(func() { SetButtonCaps(theme.Default()) })

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	w := DrawDialogButton(screen, 0, 0, "OK", 'O', false, false, false, th)
	if w != 6 || DialogButtonWidth("OK") != 6 {
		t.Fatalf("drawn %d, measured %d, want 6", w, DialogButtonWidth("OK"))
	}
	row := ""
	for x := range w {
		str, _, _ := screen.Get(x, 0)
		row += str
	}
	if row != "▎ OK ▕" {
		t.Fatalf("row = %q", row)
	}
	_, style, _ := screen.Get(0, 0)
	_, bgBase, _ := th.DialogButtonInactive.Decompose()
	if fg, bg, _ := style.Decompose(); fg != tcell.ColorRed || bg != bgBase {
		t.Fatalf("start cap fg/bg = %v/%v, want red over the button bg %v", fg, bg, bgBase)
	}
	if _, style, _ := screen.Get(1, 0); style != th.DialogButtonInactive { // cap space: button face
		t.Fatalf("cap space style = %v, want the button style", style)
	}
}
