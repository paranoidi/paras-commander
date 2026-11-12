package draw

import (
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// buttonCaps is [start, end] of the applied theme ([icons.dialog.button]). Layout measures
// buttons without a theme in hand, so the app sets the caps once per theme apply (SetButtonCaps)
// and drawing reads the same pair: measured and drawn widths always agree.
// ponytail: process-wide like SetListScroll; thread theme.Theme through dlg.Place if two themes
// ever need to be laid out at once.
var buttonCaps atomic.Pointer[[2]string]

var defaultButtonCaps = sync.OnceValue(func() [2]string {
	start, end := theme.Default().DialogButtonCaps()
	return [2]string{start, end}
})

// SetButtonCaps makes t's [icons.dialog.button] caps the ones every dialog button is drawn and
// measured with.
func SetButtonCaps(t theme.Theme) {
	start, end := t.DialogButtonCaps()
	buttonCaps.Store(&[2]string{start, end})
}

func activeButtonCaps() (start, end string) {
	if p := buttonCaps.Load(); p != nil {
		return p[0], p[1]
	}
	c := defaultButtonCaps()
	return c[0], c[1]
}

func dialogButtonWidth(label string) int {
	start, end := activeButtonCaps()
	return utf8.RuneCountInString(start) + utf8.RuneCountInString(label) + utf8.RuneCountInString(end)
}

// DialogButtonWidth returns the rune width occupied by DrawDialogButton for label.
func DialogButtonWidth(label string) int {
	return dialogButtonWidth(label)
}

// capStyle is base with cap's fg and bg, each when cap sets it.
func capStyle(base, cap tcell.Style) tcell.Style {
	fg, bg, _ := cap.Decompose()
	if fg != tcell.ColorDefault {
		base = base.Foreground(fg)
	}
	if bg != tcell.ColorDefault {
		base = base.Background(bg)
	}
	return base
}

// drawCap paints cap one rune per cell from x and returns the x after it: spaces in the button
// face (base), other runes in capStyle (edge glyphs such as pill half-circles).
func drawCap(screen tcell.Screen, x, y int, s string, base, cap tcell.Style) int {
	glyph := capStyle(base, cap)
	for _, r := range s {
		style := glyph
		if r == ' ' {
			style = base
		}
		screen.SetContent(x, y, r, nil, style)
		x++
	}
	return x
}

// DrawDialogButton renders a single button with its shortcut letter highlighted.
// shortcut is the letter inside label to highlight (e.g. 'O' for "OK").
// Output shape: start cap, label, end cap ([icons.dialog.button]), all on the state's background.
// Returns the rendered width in rune columns.
func DrawDialogButton(screen tcell.Screen, x, y int, label string, shortcut rune, focused, destructive, disabled bool, styles theme.Theme) int {
	baseStyle, caps := styles.DialogButtonInactive, styles.DialogButtonInactiveCaps
	switch {
	case focused && destructive && !disabled:
		baseStyle, caps = styles.DialogButtonActiveDestructive, styles.DialogButtonActiveDestructiveCaps
	case focused: // disabled buttons remain focusable; show active chrome while focused
		baseStyle, caps = styles.DialogButtonActive, styles.DialogButtonActiveCaps
	}
	startCap, endCap := activeButtonCaps()
	out := drawCap(screen, x, y, startCap, baseStyle, caps.Start)

	highlighted := false
	for _, r := range label {
		style := baseStyle
		if !highlighted && (r == shortcut || r == unicode.ToUpper(shortcut) || r == unicode.ToLower(shortcut)) {
			style = AccentIconStyle(baseStyle, styles.DialogAccent)
			highlighted = true
		}
		screen.SetContent(out, y, r, nil, style)
		out++
	}

	out = drawCap(screen, out, y, endCap, baseStyle, caps.End)
	return out - x
}
