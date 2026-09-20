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

// TokenColor returns a Chroma token type's foreground as a tcell color.
// ok is false when the style is missing or the token defines no foreground.
func TokenColor(styleName string, tok chroma.TokenType) (fg tcell.Color, ok bool) {
	name := strings.TrimSpace(styleName)
	if name == "" {
		return tcell.ColorDefault, false
	}
	style := chromastyles.Get(name)
	if style == nil {
		return tcell.ColorDefault, false
	}
	entry := style.Get(tok)
	if !entry.Colour.IsSet() {
		return tcell.ColorDefault, false
	}
	return chromaColourToTcell(entry.Colour), true
}

// TokenFrameStyle tints frame's foreground with a Chroma token's color, keeping frame's
// background and attributes — used for chrome painted over a syntax-tinted surface (e.g. a
// scrollbar rail icon in the Comment tint, the activity spinner in the LiteralNumber tint).
// Returns frame unchanged when the style or token color is unavailable.
func TokenFrameStyle(frame tcell.Style, styleName string, tok chroma.TokenType) tcell.Style {
	fg, ok := TokenColor(styleName, tok)
	if !ok {
		return frame
	}
	return frame.Foreground(fg)
}

func chromaColourToTcell(c chroma.Colour) tcell.Color {
	return tcell.NewRGBColor(int32(c.Red()), int32(c.Green()), int32(c.Blue()))
}
