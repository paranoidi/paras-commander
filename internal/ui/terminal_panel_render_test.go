package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func newTerminalPanelTestModel(termPanel TerminalPanelState) Model {
	return Model{
		Primary:       panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:     panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:   PrimaryPanel,
		TerminalPanel: termPanel,
	}
}

// stubTerminalDrawer paints a fixed marker on the panel's first content row.
type stubTerminalDrawer struct{ marker string }

func (d *stubTerminalDrawer) DrawTo(setCell func(x, y int, r rune, style tcell.Style)) (int, int, bool) {
	for i, r := range d.marker {
		setCell(i, 0, r, tcell.StyleDefault)
	}
	return 0, 0, false
}

// panelTopY is where the 5-row panel starts on an 80x20 screen: height(20) - footer(1) - rows(5).
const panelTopY = 14

func TestRenderDrawsTerminalPanelWhenVisible(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	drawer := &stubTerminalDrawer{marker: "shell$"}
	model := newTerminalPanelTestModel(TerminalPanelState{Visible: true, Rows: 5, Drawer: drawer})
	styles := theme.Default()
	Render(screen, model, styles)

	topRow := tcelltest.TextAt(screen, 0, panelTopY, 80)
	if !strings.Contains(topRow, drawer.marker) {
		t.Fatalf("panel top row %d = %q, want drawer marker %q", panelTopY, topRow, drawer.marker)
	}

	// Blank cells use the terminal text style background.
	_, contentStyle, _ := screen.Get(40, panelTopY+1)
	if contentStyle != styles.TerminalTextStyle() {
		t.Fatalf("content row style = %v, want %v", contentStyle, styles.TerminalTextStyle())
	}
}

func TestRenderOmitsTerminalPanelWhenHidden(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	drawer := &stubTerminalDrawer{marker: "shell$"}
	model := newTerminalPanelTestModel(TerminalPanelState{Visible: false, Rows: 5, Drawer: drawer})
	styles := theme.Default()
	Render(screen, model, styles)

	// The rows the panel would occupy belong to the file panel instead.
	for y := panelTopY; y < 19; y++ {
		if row := tcelltest.TextAt(screen, 0, y, 80); strings.Contains(row, drawer.marker) {
			t.Fatalf("row %d = %q, want no terminal panel content when hidden", y, row)
		}
	}
}

func TestRenderRelocatesTransientMessageOntoTerminalPanelTopRow(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	model := newTerminalPanelTestModel(TerminalPanelState{Visible: true, Rows: 5})
	model.Message = "hello world"
	model.MessageUrgency = MessageUrgencyInfo
	styles := theme.Default()
	Render(screen, model, styles)

	topRow := tcelltest.TextAt(screen, 0, panelTopY, 80)
	if !strings.Contains(topRow, "hello world") {
		t.Fatalf("panel top row %d = %q, want transient message painted there", panelTopY, topRow)
	}
	lastContentRow := tcelltest.TextAt(screen, 0, 18, 80)
	if strings.Contains(lastContentRow, "hello world") {
		t.Fatalf("row 18 (panel's last content row) = %q, want message NOT painted there", lastContentRow)
	}
}

func TestRenderSubFocusSuppressedWhileTerminalFocused(t *testing.T) {
	model := newTerminalPanelTestModel(TerminalPanelState{Visible: true, Focused: true, Rows: 5})
	if got := model.renderSubFocus(); got != -1 {
		t.Fatalf("renderSubFocus() with focused terminal = %d, want -1", got)
	}
	model.TerminalPanel.Focused = false
	if got := model.renderSubFocus(); got != SubFocusFileList {
		t.Fatalf("renderSubFocus() with unfocused terminal = %d, want SubFocusFileList", got)
	}
}
