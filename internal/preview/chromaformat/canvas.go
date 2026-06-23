package chromaformat

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
)

// BackgroundColors returns the Chroma Background token's foreground and background as tcell colors.
// ok is false when the style is missing or Background defines no colors.
func BackgroundColors(styleName string) (bg, fg tcell.Color, ok bool) {
	name := strings.TrimSpace(styleName)
	if name == "" {
		return tcell.ColorDefault, tcell.ColorDefault, false
	}
	style := chromastyles.Get(name)
	if style == nil {
		return tcell.ColorDefault, tcell.ColorDefault, false
	}
	entry := style.Get(chroma.Background)
	hasFG := entry.Colour.IsSet()
	hasBG := entry.Background.IsSet()
	if !hasFG && !hasBG {
		return tcell.ColorDefault, tcell.ColorDefault, false
	}
	if hasBG {
		bg = chromaColourToTcell(entry.Background)
	}
	if hasFG {
		fg = chromaColourToTcell(entry.Colour)
	}
	return bg, fg, true
}

// FrameStyleFromChroma applies Chroma Background colors to a panel frame style (borders only).
func FrameStyleFromChroma(themeFrame tcell.Style, styleName string) tcell.Style {
	chromaBG, chromaFG, ok := BackgroundColors(styleName)
	if !ok {
		return themeFrame
	}
	out := themeFrame
	if chromaBG != tcell.ColorDefault {
		out = out.Background(chromaBG)
	}
	if chromaFG != tcell.ColorDefault {
		out = out.Foreground(chromaFG)
	}
	return out
}

func chromaColourToTcell(c chroma.Colour) tcell.Color {
	return tcell.NewRGBColor(int32(c.Red()), int32(c.Green()), int32(c.Blue()))
}
