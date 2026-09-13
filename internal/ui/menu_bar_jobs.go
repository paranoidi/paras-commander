package ui

import (
	"math"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// MenuBarJobGroup is one status group in the menu-bar jobs strip: a glyph plus a decimal count.
type MenuBarJobGroup struct {
	Status string
	Count  int
}

// MenuBarJobsStrip is a snapshot for the menu-bar jobs gap (status groups + optional progress bar).
type MenuBarJobsStrip struct {
	Groups       []MenuBarJobGroup
	ProgressFrac float64
	HasProgress  bool
	// ProgressIndeterminate means totals are unknown or still provisional (streaming pre-scan), so
	// ProgressFrac is meaningless; the bar renders indeterminate: an empty track with the gradient
	// ping-ponging across it.
	ProgressIndeterminate bool
	// LightbarHead is the light-bar gradient's frame counter: the done-span cell index of the
	// head (MenuProgressDoneGfx[0]), trailing gradient cells behind it. The caller advances it
	// one per animation frame and resets it to 0 when DrawMenuBarJobsGap reports lightbarExited.
	LightbarHead int
}

const menuBarJobsProgressMinWidth = 3

// MenuBarJobsGroupsWidth measures the cell width of the rendered group strip: each group is
// "<glyph> <count>", groups separated by one space.
func MenuBarJobsGroupsWidth(groups []MenuBarJobGroup, styles theme.Theme) int {
	w := 0
	for i, g := range groups {
		if i > 0 {
			w++
		}
		glyph := styles.SymbolMenuJob(g.Status)
		w += runewidth.RuneWidth(glyph) + 1 + decimalDigits(g.Count)
	}
	return w
}

func decimalDigits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

// LayoutMenuBarJobsStrip decides the group-strip width and progress width inside totalWidth cells.
// stripWidth is the measured cell width of the group strip (0 when there are no groups).
func LayoutMenuBarJobsStrip(totalWidth, stripWidth int, wantProgress bool) (queueW, progW int) {
	if totalWidth <= 0 {
		return 0, 0
	}
	if wantProgress && stripWidth > 0 {
		if totalWidth >= stripWidth+1+menuBarJobsProgressMinWidth {
			progW = totalWidth - stripWidth - 1
			if progW >= menuBarJobsProgressMinWidth {
				return stripWidth, progW
			}
		}
		if stripWidth <= totalWidth {
			return stripWidth, 0
		}
		if wantProgress && totalWidth >= menuBarJobsProgressMinWidth {
			return 0, totalWidth
		}
		return 0, 0
	}
	if wantProgress && stripWidth == 0 {
		if totalWidth >= menuBarJobsProgressMinWidth {
			return 0, totalWidth
		}
		return 0, 0
	}
	if stripWidth > totalWidth {
		return 0, 0
	}
	return stripWidth, 0
}

// DrawMenuBarJobsGap clears the span with the menu bar background, then paints queue / progress.
// It returns lightbarExited: the light-bar gradient has completed its pass (slid past the done
// edge, or bounced back to cell 0 when indeterminate); the caller resets LightbarHead to 0 and
// repaints.
func DrawMenuBarJobsGap(screen tcell.Screen, y, startX, totalWidth int, strip MenuBarJobsStrip, styles theme.Theme) bool {
	if totalWidth <= 0 {
		return false
	}
	for i := 0; i < totalWidth; i++ {
		screen.SetContent(startX+i, y, ' ', nil, styles.MenuBarInactive)
	}
	wantProgress := strip.HasProgress && strip.ProgressFrac >= 0 && strip.ProgressFrac <= 1
	if len(strip.Groups) == 0 && !wantProgress {
		return false
	}
	stripWidth := MenuBarJobsGroupsWidth(strip.Groups, styles)
	queueW, progW := LayoutMenuBarJobsStrip(totalWidth, stripWidth, wantProgress)
	x := startX
	end := startX + totalWidth
	if queueW > 0 {
		for i, g := range strip.Groups {
			if x >= end {
				break
			}
			if i > 0 {
				screen.SetContent(x, y, ' ', nil, styles.MenuBarInactive)
				x++
				if x >= end {
					break
				}
			}
			style := styles.MenuJobStyle(g.Status)
			glyph := styles.SymbolMenuJob(g.Status)
			screen.SetContent(x, y, glyph, nil, style)
			x += runewidth.RuneWidth(glyph)
			if x >= end {
				break
			}
			screen.SetContent(x, y, ' ', nil, style)
			x++
			for _, d := range strconv.Itoa(g.Count) {
				if x >= end {
					break
				}
				screen.SetContent(x, y, d, nil, style)
				x++
			}
		}
	}
	if queueW > 0 && progW > 0 && x < end {
		screen.SetContent(x, y, ' ', nil, styles.MenuBarInactive)
		x++
	}
	doneSym := styles.SymbolMenuProgressDone()
	remSym := styles.SymbolMenuProgressRemaining()
	doneStyle := styles.MenuProgressDone
	remStyle := styles.MenuProgressRemaining
	gfx := styles.MenuProgressDoneGfx

	if strip.ProgressIndeterminate {
		// Empty track; the gradient (gfx colours, then the done colour as the last trail cell —
		// so a theme without gfx still gets a single bouncing done-coloured cell) ping-pongs
		// between the bar's ends.
		grad := append(append(make([]tcell.Style, 0, len(gfx)+1), gfx...), doneStyle)
		period := max(2*(progW-1), 1)
		p := strip.LightbarHead % period
		head, right := p, true
		if p >= progW {
			head, right = period-p, false
		}
		for i := 0; i < progW && x < end; i++ {
			k := head - i
			if !right {
				k = i - head
			}
			if k >= 0 && k < len(grad) {
				screen.SetContent(x, y, doneSym, nil, grad[k])
			} else {
				screen.SetContent(x, y, remSym, nil, remStyle)
			}
			x++
		}
		// Exited once the head is back at cell 0 after a full bounce; the caller resets to 0
		// (the same frame), keeping the counter bounded.
		return strip.LightbarHead >= period
	}

	cutoff := menuBarProgressCutoff(strip.ProgressFrac, progW)
	for i := 0; i < progW && x < end; i++ {
		switch k := strip.LightbarHead - i; {
		case i < cutoff && k >= 0 && k < len(gfx):
			screen.SetContent(x, y, doneSym, nil, gfx[k])
		case i < cutoff:
			screen.SetContent(x, y, doneSym, nil, doneStyle)
		default:
			screen.SetContent(x, y, remSym, nil, remStyle)
		}
		x++
	}
	// Exited once no gradient cell is over the done span any more; the wrap is the caller's so
	// the head never jumps when cutoff grows (a modulus on cutoff would).
	return len(gfx) > 0 && strip.LightbarHead >= cutoff+len(gfx)-1
}

// menuBarProgressCutoff returns the number of filled (done) cells for frac across progW cells.
func menuBarProgressCutoff(frac float64, progW int) int {
	if progW <= 0 {
		return 0
	}
	if frac <= 0 {
		return 0
	}
	if frac >= 1 {
		return progW
	}
	cutoff := int(math.Round(frac * float64(progW)))
	if cutoff < 0 {
		cutoff = 0
	}
	if cutoff > progW {
		cutoff = progW
	}
	return cutoff
}
