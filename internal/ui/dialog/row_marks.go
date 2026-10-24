package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// RowMarks is the pin / in-progress-job / missing state of one list row's trailing suffix icons;
// the type, its width and its painter live in the dlg engine, which draws them for list rows.
type RowMarks = dlg.RowMarks

// RowMarksFunc resolves RowMarks for one row's absolute path.
type RowMarksFunc func(absPath string) RowMarks

// rowMarksMaxWidth is RowMarksWidth's ceiling.
const rowMarksMaxWidth = dlg.RowMarksMaxWidth

// RowMarksWidth returns how many trailing cells m's active marks need.
func RowMarksWidth(m RowMarks) int { return dlg.RowMarksWidth(m) }

// DrawRowMarksSuffix blank-fills [x, x+maxWidth) to bg and paints m's marks; it returns the
// cells used.
func DrawRowMarksSuffix(screen tcell.Screen, x, y, maxWidth int, m RowMarks, bg tcell.Color, styles theme.Theme) int {
	return dlg.DrawRowMarksSuffix(screen, x, y, maxWidth, m, bg, styles)
}

// rowMarksPinRune is the single-rune-wide pin glyph of the suffix slot.
func rowMarksPinRune(styles theme.Theme) rune { return styles.IconPinRune() }
