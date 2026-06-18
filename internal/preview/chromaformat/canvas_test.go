package chromaformat_test

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
)

func TestBackgroundColorsMonokai(t *testing.T) {
	bg, _, ok := chromaformat.BackgroundColors("monokai")
	if !ok {
		t.Fatal("expected monokai Background colors")
	}
	r, g, b := rgb(bg)
	if r != 0x27 || g != 0x28 || b != 0x22 {
		t.Fatalf("monokai bg = #%02x%02x%02x, want #272822", r, g, b)
	}
}

func TestBackgroundColorsGithubLight(t *testing.T) {
	bg, _, ok := chromaformat.BackgroundColors("github")
	if !ok {
		t.Fatal("expected github Background colors")
	}
	r, g, b := rgb(bg)
	if r < 0xf0 || g < 0xf0 || b < 0xf0 {
		t.Fatalf("github bg = #%02x%02x%02x, want light canvas", r, g, b)
	}
}

func TestBackgroundColorsSolarizedDark(t *testing.T) {
	bg, fg, ok := chromaformat.BackgroundColors("solarized-dark")
	if !ok {
		t.Fatal("expected solarized-dark Background colors")
	}
	br, bgC, bb := rgb(bg)
	if br != 0x00 || bgC != 0x2b || bb != 0x36 {
		t.Fatalf("solarized-dark bg = #%02x%02x%02x, want #002b36", br, bgC, bb)
	}
	fr, fgC, fb := rgb(fg)
	if fr != 0x93 || fgC != 0xa1 || fb != 0xa1 {
		t.Fatalf("solarized-dark fg = #%02x%02x%02x, want #93a1a1", fr, fgC, fb)
	}
}

func TestBackgroundColorsEmptyName(t *testing.T) {
	if _, _, ok := chromaformat.BackgroundColors(""); ok {
		t.Fatal("empty style name should not ok")
	}
}

func TestFrameStyleFromChromaAppliesBackground(t *testing.T) {
	themeFrame := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	out := chromaformat.FrameStyleFromChroma(themeFrame, "monokai")
	_, bg, _ := out.Decompose()
	r, g, b := rgb(bg)
	if r != 0x27 || g != 0x28 || b != 0x22 {
		t.Fatalf("frame bg = #%02x%02x%02x, want monokai canvas", r, g, b)
	}
}

func rgb(c tcell.Color) (r, g, b int) {
	cr, cg, cb := c.RGB()
	return int(cr), int(cg), int(cb)
}
