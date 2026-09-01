package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// RowMarks carries the pin / in-progress-job state for one dialog or view list row's
// trailing suffix glyphs. It is plain data: this package cannot import internal/ui (ui
// imports dialog, not the reverse), so the actual Model.PinnedItems/JobPathMarks lookups
// happen in internal/ui and are handed down as a RowMarksFunc closure.
type RowMarks struct {
	Pinned    bool
	HasJob    bool
	JobStatus string
	JobWrite  bool
}

// RowMarksFunc resolves RowMarks for one row's absolute path.
type RowMarksFunc func(absPath string) RowMarks

// RowMarksWidth returns how many trailing cells m's active marks need: 2 cells per active
// mark (" <glyph>"), the same convention as panellist.SuffixDecorationLen.
// rowMarksMaxWidth is RowMarksWidth's ceiling: two possible marks (job, pin) at 2 cells each.
const rowMarksMaxWidth = 4

func RowMarksWidth(m RowMarks) int {
	n := 0
	if m.HasJob {
		n += 2
	}
	if m.Pinned {
		n += 2
	}
	return n
}

// rowMarksPinRune returns styles.SymbolPinRune() for the single-rune-wide suffix slot,
// mirroring panellist's pinSymbolRune.
func rowMarksPinRune(styles theme.Theme) rune {
	return styles.SymbolPinRune()
}

// DrawRowMarksSuffix blank-fills [x, x+maxWidth) to bg, then paints the job glyph
// (Theme.SymbolFilelistJob, styled via Theme.PanelJobMarkStyle) followed by the pin glyph
// (Theme.SymbolPin, styled via Theme.PanelRowMarkPinned) within maxWidth. Job-before-pin
// matches panellist.EntryDisplayRunes's suffix order exactly, so the look is identical to the
// main panel's row marks. Returns cells used.
func DrawRowMarksSuffix(screen tcell.Screen, x, y, maxWidth int, m RowMarks, bg tcell.Color, styles theme.Theme) int {
	if maxWidth <= 0 {
		return 0
	}
	primitive.Text(screen, x, y, maxWidth, "", tcell.StyleDefault.Background(bg))
	used := 0
	if m.HasJob && used+2 <= maxWidth {
		fg, _, _ := styles.PanelJobMarkStyle(m.JobStatus, m.JobWrite).Decompose()
		primitive.Text(screen, x+used, y, 2, " "+string(styles.SymbolFilelistJob()), tcell.StyleDefault.Foreground(fg).Background(bg))
		used += 2
	}
	if m.Pinned && used+2 <= maxWidth {
		fg, _, _ := styles.PanelRowMarkPinned.Decompose()
		primitive.Text(screen, x+used, y, 2, " "+string(rowMarksPinRune(styles)), tcell.StyleDefault.Foreground(fg).Background(bg))
		used += 2
	}
	return used
}
