package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestFilePreviewFrameStyleUsesChromaBackground(t *testing.T) {
	styles := theme.Default()
	frame := filePreviewFrameStyle(styles, false, false, false, "github")
	_, bg, _ := frame.Decompose()
	r, g, b := rgb(bg)
	if r < 0xf0 || g < 0xf0 || b < 0xf0 {
		t.Fatalf("frame bg = #%02x%02x%02x, want light github canvas", r, g, b)
	}
}

func TestFilePreviewFrameStyleKeepsChromaWhenBlocked(t *testing.T) {
	styles := theme.Default()
	want := chromaformat.FrameStyleFromChroma(styles.PanelBlockedFrame, "github")
	got := filePreviewFrameStyle(styles, false, true, false, "github")
	if got != want {
		t.Fatal("blocked preview should keep the chroma tint so border/pad background stays consistent with the already-rendered body")
	}
}

func TestFilePreviewFrameStyleSkipsChromaWhenStyleEmpty(t *testing.T) {
	styles := theme.Default()
	want := styles.PanelInactiveFrame
	got := filePreviewFrameStyle(styles, false, false, false, "")
	if got != want {
		t.Fatal("empty chroma style should keep theme frame")
	}
}

func TestFilePreviewFrameStyleMatchesChromaHelper(t *testing.T) {
	styles := theme.Default()
	themeFrame := styles.PanelInactiveFrame
	want := chromaformat.FrameStyleFromChroma(themeFrame, "monokai")
	got := filePreviewFrameStyle(styles, false, false, false, "monokai")
	if got != want {
		t.Fatal("filePreviewFrameStyle should delegate to chromaformat.FrameStyleFromChroma")
	}
}

func rgb(c tcell.Color) (r, g, b int) {
	cr, cg, cb := c.RGB()
	return int(cr), int(cg), int(cb)
}

func TestDrawFilePreviewPanelEmbeddedScrollbarRailUsesOverrideNotChromaFrame(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	// One column wider than the preview rect itself, mirroring the real carousel case
	// where the true panel border sits one column past the child preview's own rect.
	const panelWidth, panelHeight, screenWidth = 40, 10, 41
	screen.SetSize(screenWidth, panelHeight)

	styles := theme.Default()
	// A chroma style is set so the embedded frame would normally be Chroma-tinted (see
	// filePreviewFrameStyle); the panel border override must win over that tint for the rail.
	st := FilePreviewState{
		Open:         true,
		TitleBase:    "sample.go",
		ChromaStyle:  "monokai",
		CombinedText: strings.Repeat("line\n", 20),
	}
	borderOverride := tcell.StyleDefault.Foreground(tcell.NewRGBColor(0x11, 0x22, 0x33))
	scrollGutterX := panelWidth // one column past this rect's own right edge, like the real carousel border column

	drawFilePreviewPanel(screen, Rect{X: 0, Y: 0, Width: panelWidth, Height: panelHeight}, st, styles,
		false, false, false, true, false, "", "", uiscrollbar.StyleThumb, scrollGutterX, borderOverride)

	found := false
	for row := 0; row < panelHeight-1; row++ {
		c, gotStyle, _ := screen.Get(scrollGutterX, row+1)
		if c == "│" {
			found = true
			if gotStyle != borderOverride {
				t.Fatalf("rail style at row %d = %v, want panel border override %v (not chroma-tinted frame)", row, gotStyle, borderOverride)
			}
		}
	}
	if !found {
		t.Fatal("no plain rail glyph found at the overridden gutter column")
	}
}
