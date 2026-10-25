package ui

import (
	"slices"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const (
	jobsConflictLabelNew      = "New:      "
	jobsConflictLabelExisting = "Existing: "
	jobsConflictLabelSize     = "Size:     "
	jobsConflictLabelModified = "Modified: "
)

// JobEntryShowsConflictPanel reports whether the jobs blocker UI should be shown for this row.
func JobEntryShowsConflictPanel(j JobEntry) bool {
	return j.Status == string(jobs.StatusWaitingDecision) && j.PendingBlocker != nil
}

// FirstJobEntryWaitingDecisionIndex returns the list index of the first job awaiting
// blocker input, or -1 when none.
func FirstJobEntryWaitingDecisionIndex(entries []JobEntry) int {
	for i, e := range entries {
		if JobEntryShowsConflictPanel(e) {
			return i
		}
	}
	return -1
}

// JobsBlockerMaxButtonIndex returns the maximum focused button index for the blocker panel.
func JobsBlockerMaxButtonIndex(j JobEntry) int {
	if j.PendingBlocker == nil {
		return 5
	}
	if j.PendingBlocker.Kind == jobs.BlockerKindDiskSpace {
		return 1
	}
	return 5
}

// JobsPane names a focusable panel of the jobs view. JobsViewState.FocusPane indexes the
// panels currently shown, in JobsPaneOrder.
type JobsPane int

const (
	JobsPaneList JobsPane = iota
	JobsPaneConflict
	JobsPaneDetail
	JobsPaneErrors
	JobsPaneSucceeded
)

// JobsPaneOrder is the Tab focus order of the shown panels: list, blocker (when shown), Details
// (always), Errors (when shown), Succeeded (when shown).
func JobsPaneOrder(showConflict, showErrors, showSucceeded bool) []JobsPane {
	order := []JobsPane{JobsPaneList}
	if showConflict {
		order = append(order, JobsPaneConflict)
	}
	order = append(order, JobsPaneDetail)
	if showErrors {
		order = append(order, JobsPaneErrors)
	}
	if showSucceeded {
		order = append(order, JobsPaneSucceeded)
	}
	return order
}

// focusedJobsPane returns the panel named by state.FocusPane; out-of-range indices name the list.
func focusedJobsPane(state JobsViewState, order []JobsPane) JobsPane {
	if state.FocusPane < 0 || state.FocusPane >= len(order) {
		return JobsPaneList
	}
	return order[state.FocusPane]
}

// blockerMismatch reports a directory/non-directory conflict, where the first button pair is
// Rename / Rename All instead of Overwrite / Overwrite All and Advanced… is absent.
func blockerMismatch(b *jobs.BlockerDetails) bool {
	return b != nil && b.Kind != jobs.BlockerKindDiskSpace && b.Conflict != nil && b.Conflict.Mismatch
}

// conflictPanelButtons are the panel buttons in focus-index order; the dialog's IDs carry the
// decision (dialog.ConflictDecisionOf), and Advanced… opens the rules dialog instead.
var (
	conflictPanelButtons         = [...]dlg.ID{dialog.ConflictOverwrite, dialog.ConflictOverwriteAll, dialog.ConflictAdvanced, dialog.ConflictSkip, dialog.ConflictSkipAll, dialog.ConflictCancelJob}
	conflictPanelMismatchButtons = [...]dlg.ID{dialog.ConflictRename, dialog.ConflictRenameAll, dialog.ConflictAdvanced, dialog.ConflictSkip, dialog.ConflictSkipAll, dialog.ConflictCancelJob}
)

// ConflictDecisionFromButtonIndex maps conflict panel button order to domain decisions.
func ConflictDecisionFromButtonIndex(idx int, mismatch bool) jobs.ConflictDecision {
	buttons := conflictPanelButtons
	if mismatch {
		buttons = conflictPanelMismatchButtons
	}
	idx = min(max(idx, 0), len(buttons)-1)
	d, _ := dialog.ConflictDecisionOf(buttons[idx]) // Advanced… submits no decision: ""
	return d
}

// conflictAdvancedFocus is the button index of "Advanced…" in the conflict panel.
const conflictAdvancedFocus = 2

// JobBlockerPanelIsAdvanced reports whether panel button focus selects "Advanced…" (not a decision).
func JobBlockerPanelIsAdvanced(sel JobEntry, btnIdx int) bool {
	return sel.PendingBlocker != nil && sel.PendingBlocker.Kind != jobs.BlockerKindDiskSpace && !blockerMismatch(sel.PendingBlocker) && btnIdx == conflictAdvancedFocus
}

// JobBlockerDecisionFromFocus resolves the decision for the blocker pane from button focus.
func JobBlockerDecisionFromFocus(sel JobEntry, btnIdx int) jobs.ConflictDecision {
	if sel.PendingBlocker != nil && sel.PendingBlocker.Kind == jobs.BlockerKindDiskSpace {
		if btnIdx <= 0 {
			return jobs.DecisionRetry
		}
		return jobs.DecisionCancel
	}
	return ConflictDecisionFromButtonIndex(btnIdx, blockerMismatch(sel.PendingBlocker))
}

// moveFocusInDiskRow applies navigation keys to the disk-space blocker pane buttons: Tab wraps,
// Left/Right stop at the ends, Up/Down are consumed.
func moveFocusInDiskRow(focus int, key tcell.Key) (int, bool) {
	const maxFocus = 2
	focus = min(max(focus, 0), maxFocus)
	switch key {
	case tcell.KeyTab:
		if focus >= maxFocus {
			return 0, true
		}
		return focus + 1, true
	case tcell.KeyBacktab:
		if focus <= 0 {
			return maxFocus, true
		}
		return focus - 1, true
	case tcell.KeyLeft:
		return max(focus-1, 0), true
	case tcell.KeyRight:
		return min(focus+1, maxFocus), true
	case tcell.KeyUp, tcell.KeyDown:
		return focus, true
	}
	return focus, false
}

// Conflict button rows as focus indexes (see ConflictDecisionFromButtonIndex).
var (
	conflictPanelButtonGrid = [][]int{{0, 1, 2}, {3, 4, 5}}
	// A directory mismatch has no Advanced… button (focus 2).
	conflictPanelMismatchGrid = [][]int{{0, 1}, {3, 4, 5}}
)

// JobsBlockerPanelMoveFocus applies navigation keys to the jobs-view blocker pane buttons,
// which lay out like the quick dialog minus Postpone.
func JobsBlockerPanelMoveFocus(b jobs.BlockerDetails, focus int, key tcell.Key) (int, bool) {
	if b.Kind == jobs.BlockerKindDiskSpace {
		return moveFocusInDiskRow(focus, key)
	}
	if blockerMismatch(&b) {
		return moveFocusInButtonGrid(conflictPanelMismatchGrid, focus, key)
	}
	return moveFocusInButtonGrid(conflictPanelButtonGrid, focus, key)
}

// moveFocusInButtonGrid moves focus through centered button rows: Tab/Left/Right step in
// reading order (Tab wraps, arrows do not), Up/Down jump to the visually nearest button.
func moveFocusInButtonGrid(grid [][]int, focus int, key tcell.Key) (int, bool) {
	var flat []int
	row, col := 0, 0
	for r, cells := range grid {
		for c, f := range cells {
			if f == focus {
				row, col = r, c
			}
			flat = append(flat, f)
		}
	}
	pos := slices.Index(flat, focus)
	if pos < 0 {
		return flat[0], true
	}
	switch key {
	case tcell.KeyTab:
		return flat[(pos+1)%len(flat)], true
	case tcell.KeyBacktab:
		return flat[(pos+len(flat)-1)%len(flat)], true
	case tcell.KeyLeft:
		return flat[max(pos-1, 0)], true
	case tcell.KeyRight:
		return flat[min(pos+1, len(flat)-1)], true
	case tcell.KeyUp, tcell.KeyDown:
		next := row - 1
		if key == tcell.KeyDown {
			next = row + 1
		}
		if next < 0 || next >= len(grid) {
			return focus, true
		}
		src, dst := len(grid[row]), len(grid[next])
		return grid[next][(2*col+1)*dst/(2*src)], true
	default:
		return focus, false
	}
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
	layout := drawAuxPanelChrome(screen, rect, " Disk space ", "", focused, chromeBlocked, false, styles)
	borderStyle := layout.Chrome.Frame
	body := auxPanelBodyText(styles, chromeBlocked, layout.ContentBG)
	warn := styles.MessageWarn.Background(layout.ContentBG)

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

	row := []dialog.DialogButtonSpec{
		{Label: "Retry", Shortcut: 'R', Focused: focused && f == 0},
		{Label: "Abort", Shortcut: 'B', Focused: focused && f == 1},
	}
	if y <= rect.Y+rect.Height-2 {
		dialog.DrawDialogButtonRowCentered(screen, rect, y, row, styles)
	}
}

func drawJobsFileConflictPanel(screen tcell.Screen, rect Rect, state JobsViewState, c *jobs.ConflictEvent, styles theme.Theme, chromeBlocked, focused bool, userHomeDir string) {
	layout := drawAuxPanelChrome(screen, rect, " File exists ", "", focused, chromeBlocked, false, styles)
	borderStyle := layout.Chrome.Frame
	body := auxPanelBodyText(styles, chromeBlocked, layout.ContentBG)
	prompt := styles.DialogText.Background(layout.ContentBG)

	textW := rect.Width - 4
	if textW < 1 {
		textW = 1
	}
	textX := rect.X + 2
	y := rect.Y + 1

	y = drawJobsConflictFileGroup(screen, textX, y, textW, jobsConflictLabelNew, c.Source, c.SourceSize, c.SourceTime, body, userHomeDir)
	y++
	y = drawJobsConflictFileGroup(screen, textX, y, textW, jobsConflictLabelExisting, c.Destination, c.DestSize, c.DestTime, body, userHomeDir)

	if y < rect.Y+rect.Height-1 {
		dialog.DrawDialogHSeparator(screen, rect, y, borderStyle)
		y++
	}

	primitive.Text(screen, textX, y, textW, dialog.ConflictPromptText(c), prompt)
	y++
	y++

	f := state.ConflictButtonFocus
	if f < 0 {
		f = 0
	}
	if f > 5 {
		f = 5
	}

	row1 := []dialog.DialogButtonSpec{
		{Label: "Overwrite", Shortcut: 'O', Focused: focused && f == 0, Destructive: true},
		{Label: "Overwrite All", Shortcut: 'A', Focused: focused && f == 1, Destructive: true},
		{Label: "Advanced…", Shortcut: 'D', Focused: focused && f == 2, Destructive: true},
	}
	if c.Mismatch {
		row1 = []dialog.DialogButtonSpec{
			{Label: "Rename", Shortcut: 'R', Focused: focused && f == 0},
			{Label: "Rename All", Shortcut: 'A', Focused: focused && f == 1},
		}
	}
	row2 := []dialog.DialogButtonSpec{
		{Label: "Skip", Shortcut: 'S', Focused: focused && f == 3},
		{Label: "Skip All", Shortcut: 'L', Focused: focused && f == 4},
		{Label: "Cancel", Shortcut: 'C', Focused: focused && f == 5},
	}
	if y <= rect.Y+rect.Height-3 {
		dialog.DrawDialogButtonRowCentered(screen, rect, y, row1, styles)
		y++
	}
	if y <= rect.Y+rect.Height-2 {
		dialog.DrawDialogButtonRowCentered(screen, rect, y, row2, styles)
	}
}

func drawJobsConflictFileGroup(screen tcell.Screen, textX, y, textW int, pathLabel, path, size, modified string, body tcell.Style, userHomeDir string) int {
	pathAvail := max(1, textW-utf8.RuneCountInString(pathLabel))
	pathLine := pathLabel + primitive.FitPathForWidth(primitive.PathWithHomeTilde(path, userHomeDir), pathAvail)
	primitive.Text(screen, textX, y, textW, pathLine, body)
	y++
	primitive.Text(screen, textX, y, textW, jobsConflictLabelSize+jobsConflictDisplaySize(size), body)
	y++
	primitive.Text(screen, textX, y, textW, jobsConflictLabelModified+jobsConflictDisplayText(modified), body)
	y++
	return y
}

func jobsConflictDisplaySize(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func jobsConflictDisplayText(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
