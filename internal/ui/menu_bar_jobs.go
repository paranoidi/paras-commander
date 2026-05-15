package ui

import (
	"math"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// MenuBarJobsStrip is a snapshot for the menu-bar jobs gap (queue glyphs + optional progress bar).
type MenuBarJobsStrip struct {
	QueueStatuses []string
	ProgressFrac  float64
	HasProgress   bool
}

const menuBarJobsProgressMinWidth = 3

// LayoutMenuBarJobsStrip decides queue width and progress width inside totalWidth cells.
func LayoutMenuBarJobsStrip(totalWidth, queueLen int, wantProgress bool) (queueW, progW int) {
	if totalWidth <= 0 {
		return 0, 0
	}
	if wantProgress && queueLen > 0 {
		if totalWidth >= queueLen+1+menuBarJobsProgressMinWidth {
			progW = totalWidth - queueLen - 1
			if progW >= menuBarJobsProgressMinWidth {
				return queueLen, progW
			}
		}
		if queueLen <= totalWidth {
			return queueLen, 0
		}
		if wantProgress && totalWidth >= menuBarJobsProgressMinWidth {
			return 0, totalWidth
		}
		return 0, 0
	}
	if wantProgress && queueLen == 0 {
		if totalWidth >= menuBarJobsProgressMinWidth {
			return 0, totalWidth
		}
		return 0, 0
	}
	if queueLen > totalWidth {
		return 0, 0
	}
	return queueLen, 0
}

// DrawMenuBarJobsGap clears the span with the menu bar background, then paints queue / progress.
func DrawMenuBarJobsGap(screen tcell.Screen, y, startX, totalWidth int, strip MenuBarJobsStrip, styles theme.Theme) {
	if totalWidth <= 0 {
		return
	}
	for i := 0; i < totalWidth; i++ {
		screen.SetContent(startX+i, y, ' ', nil, styles.MenuBar)
	}
	wantProgress := strip.HasProgress && strip.ProgressFrac >= 0 && strip.ProgressFrac <= 1
	if len(strip.QueueStatuses) == 0 && !wantProgress {
		return
	}
	queueLen := len(strip.QueueStatuses)
	queueW, progW := LayoutMenuBarJobsStrip(totalWidth, queueLen, wantProgress)
	x := startX
	for i := 0; i < queueW; i++ {
		if x >= startX+totalWidth {
			break
		}
		st := strip.QueueStatuses[i]
		glyph := styles.SymbolMenuJob(st)
		screen.SetContent(x, y, glyph, nil, styles.MenuJobStyle(st))
		x++
	}
	if queueW > 0 && progW > 0 && x < startX+totalWidth {
		screen.SetContent(x, y, ' ', nil, styles.MenuBar)
		x++
	}
	doneSym := styles.SymbolMenuProgressDone()
	remSym := styles.SymbolMenuProgressRemaining()
	doneStyle := styles.MenuProgressDone
	remStyle := styles.MenuProgressRemaining
	for i := 0; i < progW && x < startX+totalWidth; i++ {
		if menuBarProgressFilledCells(strip.ProgressFrac, progW, i) {
			screen.SetContent(x, y, doneSym, nil, doneStyle)
		} else {
			screen.SetContent(x, y, remSym, nil, remStyle)
		}
		x++
	}
}

func menuBarProgressFilledCells(frac float64, progW, index int) bool {
	if progW <= 0 {
		return false
	}
	if frac <= 0 {
		return false
	}
	if frac >= 1 {
		return true
	}
	cutoff := int(math.Round(frac * float64(progW)))
	if cutoff < 0 {
		cutoff = 0
	}
	if cutoff > progW {
		cutoff = progW
	}
	return index < cutoff
}
