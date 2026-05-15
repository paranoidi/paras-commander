package ui

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// JobEntryShowsConflictPanel reports whether the jobs blocker UI should be shown for this row.
func JobEntryShowsConflictPanel(j JobEntry) bool {
	return j.Status == string(jobs.StatusWaitingDecision) && j.PendingBlocker != nil
}

// JobsBlockerMaxButtonIndex returns the maximum focused button index for the blocker panel.
func JobsBlockerMaxButtonIndex(j JobEntry) int {
	if j.PendingBlocker == nil {
		return 4
	}
	if j.PendingBlocker.Kind == jobs.BlockerKindDiskSpace {
		return 1
	}
	return 4
}

func jobsDetailPaneFocused(state JobsViewState, showConflict bool) bool {
	if showConflict {
		return state.FocusPane == 2
	}
	return state.FocusPane == 1
}

func jobsActivityPaneFocused(state JobsViewState, showConflict bool) bool {
	if showConflict {
		return state.FocusPane == 3
	}
	return state.FocusPane == 2
}

func jobsConflictPaneFocused(state JobsViewState, showConflict bool) bool {
	return showConflict && state.FocusPane == 1
}

// ConflictDecisionFromButtonIndex maps conflict panel button order to domain decisions.
func ConflictDecisionFromButtonIndex(idx int) jobs.ConflictDecision {
	switch idx {
	case 0:
		return jobs.DecisionOverwrite
	case 1:
		return jobs.DecisionSkip
	case 2:
		return jobs.DecisionOverwriteAll
	case 3:
		return jobs.DecisionSkipAll
	default:
		return jobs.DecisionCancel
	}
}

// JobBlockerDecisionFromFocus resolves the decision for the blocker pane from button focus.
func JobBlockerDecisionFromFocus(sel JobEntry, btnIdx int) jobs.ConflictDecision {
	if sel.PendingBlocker != nil && sel.PendingBlocker.Kind == jobs.BlockerKindDiskSpace {
		if btnIdx <= 0 {
			return jobs.DecisionRetry
		}
		return jobs.DecisionCancel
	}
	return ConflictDecisionFromButtonIndex(btnIdx)
}

func drawJobsConflictPanel(screen tcell.Screen, rect Rect, state JobsViewState, sel JobEntry, styles theme.Theme, chromeBlocked, focused bool, userHomeDir string) {
	b := sel.PendingBlocker
	if b == nil || rect.Height < 3 {
		return
	}
	if b.Kind == jobs.BlockerKindDiskSpace {
		drawJobsDiskSpacePanel(screen, rect, state, b, styles, chromeBlocked, focused, userHomeDir)
		return
	}
	if b.Conflict != nil {
		drawJobsFileConflictPanel(screen, rect, state, b.Conflict, styles, chromeBlocked, focused, userHomeDir)
	}
}

func drawJobsDiskSpacePanel(screen tcell.Screen, rect Rect, state JobsViewState, b *jobs.BlockerDetails, styles theme.Theme, chromeBlocked, focused bool, userHomeDir string) {
	d := b.DiskSpace
	if d == nil {
		return
	}
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if focused {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if focused {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleW := rect.Width - 4
	if titleW < 1 {
		titleW = 1
	}
	primitive.TextOverlay(screen, titleX, rect.Y, titleW, " Disk space ", titleStyle)

	body := styles.PanelRowNormal.Background(bg)
	if chromeBlocked {
		body = styles.PanelBlockedRowNormal
	}
	warn := styles.StatusWarn.Background(bg)

	textW := rect.Width - 4
	if textW < 1 {
		textW = 1
	}
	textX := rect.X + 2
	y := rect.Y + 1

	primitive.Text(screen, textX, y, textW, "Not enough free space.", warn)
	y++

	prefixVol := "Destination: "
	lineVol := prefixVol + primitive.FitPathForWidth(primitive.PathWithHomeTilde(d.Destination, userHomeDir), max(1, textW-utf8.RuneCountInString(prefixVol)))
	primitive.Text(screen, textX, y, textW, lineVol, body)
	y++

	reqLabel := "Required:    " + formatJobBytes(d.RequiredBytes)
	primitive.Text(screen, textX, y, textW, reqLabel, body)
	y++

	var availLabel string
	if d.AvailableKnown {
		availLabel = "Available:   " + formatJobBytes(int64(d.AvailableBytes))
	} else {
		availLabel = "Available:   (unknown)"
	}
	primitive.Text(screen, textX, y, textW, availLabel, body)
	y++

	if d.NextSource != "" {
		prefixNext := "Next file:   "
		lineNext := prefixNext + primitive.FitPathForWidth(primitive.PathWithHomeTilde(d.NextSource, userHomeDir), max(1, textW-utf8.RuneCountInString(prefixNext)))
		primitive.Text(screen, textX, y, textW, lineNext, body)
		y++
	}

	if y < rect.Y+rect.Height-1 {
		dialog.DrawDialogHSeparator(screen, rect, y, borderStyle)
		y++
	}

	primitive.Text(screen, textX, y, textW, "Make space or retry.", warn)
	y++

	f := state.ConflictButtonFocus
	maxF := 1
	if f < 0 {
		f = 0
	}
	if f > maxF {
		f = maxF
	}

	row := []DialogButtonSpec{
		{Label: "Retry", Shortcut: 'R', Focused: focused && f == 0},
		{Label: "Abort", Shortcut: 'B', Focused: focused && f == 1},
	}
	if y <= rect.Y+rect.Height-2 {
		dialog.DrawDialogButtonRowCentered(screen, rect, y, row, styles)
	}
}

func drawJobsFileConflictPanel(screen tcell.Screen, rect Rect, state JobsViewState, c *jobs.ConflictEvent, styles theme.Theme, chromeBlocked, focused bool, userHomeDir string) {
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if focused {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if focused {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleW := rect.Width - 4
	if titleW < 1 {
		titleW = 1
	}
	primitive.TextOverlay(screen, titleX, rect.Y, titleW, " File exists ", titleStyle)

	body := styles.PanelRowNormal.Background(bg)
	if chromeBlocked {
		body = styles.PanelBlockedRowNormal
	}
	warn := styles.StatusWarn.Background(bg)
	if chromeBlocked {
		warn = styles.StatusWarn.Background(bg)
	}

	textW := rect.Width - 4
	if textW < 1 {
		textW = 1
	}
	textX := rect.X + 2
	y := rect.Y + 1

	prefixNew := "New:      "
	lineNewPath := prefixNew + primitive.FitPathForWidth(primitive.PathWithHomeTilde(c.Source, userHomeDir), max(1, textW-utf8.RuneCountInString(prefixNew)))
	primitive.Text(screen, textX, y, textW, lineNewPath, body)
	y++
	sizeLine := fmt.Sprintf("%s %s", padOrDash(c.SourceSize, 12), padOrDash(c.SourceTime, 14))
	primitive.Text(screen, textX, y, textW, sizeLine, body)
	y++

	prefixEx := "Existing: "
	lineExPath := prefixEx + primitive.FitPathForWidth(primitive.PathWithHomeTilde(c.Destination, userHomeDir), max(1, textW-utf8.RuneCountInString(prefixEx)))
	primitive.Text(screen, textX, y, textW, lineExPath, body)
	y++
	sizeLine2 := fmt.Sprintf("%s %s", padOrDash(c.DestSize, 12), padOrDash(c.DestTime, 14))
	primitive.Text(screen, textX, y, textW, sizeLine2, body)
	y++

	if y < rect.Y+rect.Height-1 {
		dialog.DrawDialogHSeparator(screen, rect, y, borderStyle)
		y++
	}

	primitive.Text(screen, textX, y, textW, "Overwrite this file?", warn)
	y++

	f := state.ConflictButtonFocus
	if f < 0 {
		f = 0
	}
	if f > 4 {
		f = 4
	}

	row1 := []DialogButtonSpec{
		{Label: "Overwrite", Shortcut: 'O', Focused: focused && f == 0},
		{Label: "Skip", Shortcut: 'S', Focused: focused && f == 1},
		{Label: "Overwrite All", Shortcut: 'A', Focused: focused && f == 2},
	}
	row2 := []DialogButtonSpec{
		{Label: "Skip All", Shortcut: 'L', Focused: focused && f == 3},
		{Label: "Cancel", Shortcut: 'C', Focused: focused && f == 4},
	}
	if y <= rect.Y+rect.Height-3 {
		dialog.DrawDialogButtonRowCentered(screen, rect, y, row1, styles)
		y++
	}
	if y <= rect.Y+rect.Height-2 {
		dialog.DrawDialogButtonRowCentered(screen, rect, y, row2, styles)
	}
}

func padOrDash(s string, w int) string {
	r := []rune(s)
	if len(r) >= w {
		return string(r[:w])
	}
	if s == "" {
		s = "—"
		r = []rune(s)
	}
	for len(r) < w {
		r = append(r, ' ')
	}
	return string(r[:w])
}
