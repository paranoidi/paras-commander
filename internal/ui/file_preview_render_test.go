package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func TestFilePreviewFrameStyleSkipsChromaWhenBlocked(t *testing.T) {
	styles := theme.Default()
	want := styles.PanelBlockedFrame
	got := filePreviewFrameStyle(styles, false, true, false, "github")
	if got != want {
		t.Fatal("blocked preview should keep theme frame without chroma tint")
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
