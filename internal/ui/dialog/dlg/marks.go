package dlg

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// RowMarks carries the pin / in-progress-job state for one dialog or view list row's
// trailing suffix icons. It is plain data: the engine cannot import internal/ui, so the actual
// Model.PinnedItems/JobPathMarks lookups happen in internal/ui and are handed down as a
// dialog.RowMarksFunc closure; a ListRow carries the resolved value.
type RowMarks struct {
	Pinned    bool
	HasJob    bool
	JobStatus string
	JobWrite  bool
	// Missing marks a path that no longer exists (Theme.IconDialogMissing); Cursor switches
	// its color to dialog.option.active (Theme.DialogMissingIconForeground).
	Missing bool
	Cursor  bool
	// RangeAnchor marks the pending range-select start row (Theme.IconFilelistRangeAnchor).
	RangeAnchor bool
}

// RowMarksMaxWidth is RowMarksWidth's ceiling: four possible marks (job, pin, missing, range
// anchor) at 2 cells each.
const RowMarksMaxWidth = 8

// RowMarksWidth returns how many trailing cells m's active marks need: 2 cells per active
// mark (" <icon>"), the same convention as panellist.SuffixDecorationLen.
func RowMarksWidth(m RowMarks) int {
	n := 0
	if m.HasJob {
		n += 2
	}
	if m.Pinned {
		n += 2
	}
	if m.Missing {
		n += 2
	}
	if m.RangeAnchor {
		n += 2
	}
	return n
}

// rowMarksPinRune returns styles.IconPinRune() for the single-rune-wide suffix slot,
// mirroring panellist's pinIconRune.
func rowMarksPinRune(styles theme.Theme) rune {
	return styles.IconPinRune()
}

// DrawRowMarksSuffix blank-fills [x, x+maxWidth) to bg, then paints the job icon
// (Theme.IconFilelistJob, styled via Theme.PanelJobMarkStyle) followed by the pin icon
// (Theme.IconPin, styled via Theme.PanelRowMarkPinned) within maxWidth. Job-before-pin
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
		primitive.Text(screen, x+used, y, 2, " "+string(styles.IconFilelistJob()), tcell.StyleDefault.Foreground(fg).Background(bg))
		used += 2
	}
	if m.Pinned && used+2 <= maxWidth {
		fg, _, _ := styles.PanelRowMarkPinned.Decompose()
		primitive.Text(screen, x+used, y, 2, " "+string(rowMarksPinRune(styles)), tcell.StyleDefault.Foreground(fg).Background(bg))
		used += 2
	}
	if m.Missing && used+2 <= maxWidth {
		fg := styles.DialogMissingIconForeground(m.Cursor)
		primitive.Text(screen, x+used, y, 2, " "+styles.IconDialogMissing(), tcell.StyleDefault.Foreground(fg).Background(bg))
		used += 2
	}
	if m.RangeAnchor && used+2 <= maxWidth {
		fg, _, _ := styles.PanelRowMarkRangeAnchor.Decompose()
		primitive.Text(screen, x+used, y, 2, " "+string(styles.IconFilelistRangeAnchor()), tcell.StyleDefault.Foreground(fg).Background(bg))
		used += 2
	}
	return used
}
