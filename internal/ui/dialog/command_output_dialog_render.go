package dialog

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// CommandOutputDialogMetrics holds computed geometry for the output dialog.
type CommandOutputDialogMetrics struct {
	Rect         draw.Rect
	ListH        int
	ContentWidth int
}

// cmdOutputOverhead: top-border/title + separator + blank-above-button + button + bottom-border.
const cmdOutputOverhead = 5

// ParseDialogDimension parses "N%" as a percentage of total, or "N" as an absolute count.
// Returns 0 for empty or unparseable input so the caller can substitute its own default.
func ParseDialogDimension(raw string, total int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if pct, ok := strings.CutSuffix(raw, "%"); ok {
		f, err := strconv.ParseFloat(pct, 64)
		if err != nil || f <= 0 {
			return 0
		}
		return int(float64(total) * f / 100)
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// ComputeCommandOutputDialogMetrics calculates dialog geometry from layout and state.
func ComputeCommandOutputDialogMetrics(layout Layout, state CommandOutputDialogState) (m CommandOutputDialogMetrics, ok bool) {
	const minListH = 4
	const minW = 40

	prefW := ParseDialogDimension(state.PrefWidth, layout.Width)
	if prefW == 0 {
		prefW = layout.Width * 4 / 5
	}
	if prefW < minW {
		prefW = minW
	}
	maxW := layout.Width - 4
	if maxW < minW {
		return m, false
	}
	if prefW > maxW {
		prefW = maxW
	}

	prefH := ParseDialogDimension(state.PrefHeight, layout.Height)
	if prefH == 0 {
		prefH = layout.Height * 3 / 5
	}
	listH := prefH - cmdOutputOverhead
	listH = max(listH, minListH)
	height := listH + cmdOutputOverhead
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - cmdOutputOverhead
		if listH < minListH {
			return m, false
		}
	}

	rect := draw.CenteredDialogRect(layout, prefW, height)
	m.Rect = rect
	m.ListH = listH
	m.ContentWidth = draw.DialogContentWidth(rect)
	return m, true
}

// CommandOutputDialogListH returns the visible content row count for scroll handling.
// Returns 0 when the dialog would not fit the current layout.
func CommandOutputDialogListH(layout Layout, state CommandOutputDialogState) int {
	m, ok := ComputeCommandOutputDialogMetrics(layout, state)
	if !ok {
		return 0
	}
	return m.ListH
}

// DrawCommandOutputDialog renders the command output modal overlay.
func DrawCommandOutputDialog(screen tcell.Screen, layout Layout, state CommandOutputDialogState, styles theme.Theme, scrollbarStyle uiscrollbar.Style, isSelected func(string) bool) {
	if !state.Open {
		return
	}
	metrics, ok := ComputeCommandOutputDialogMetrics(layout, state)
	if !ok {
		return
	}
	rect := metrics.Rect
	listH := metrics.ListH
	contentW := metrics.ContentWidth

	title := strings.TrimSpace(state.Title)
	if title == "" {
		title = "Output"
	}
	if state.Running {
		status := styles.IconMetaRunning() + " Running…"
		if state.Queued {
			status = styles.IconJobsList("queued") + " Queued…"
		}
		title += " " + status
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	lineStyle := styles.DialogText.Background(dbg)

	contentX := draw.DialogTextX(rect)
	scroll := state.Scroll
	if state.Follow {
		scroll = max(0, len(state.Lines)-listH)
	}
	for row := range listH {
		y := rect.Y + 1 + row
		line := ""
		if idx := scroll + row; idx < len(state.Lines) {
			line = state.Lines[idx]
		}
		// ponytail: per-line SGR parse; a color left open across \n does not carry to the next
		// line. Parse the whole output once into cell rows if a tool needs that.
		cells := previewpanel.AnsiStyledCells(line, lineStyle)
		for i, m := range state.Paths {
			if m.Line != scroll+row {
				continue
			}
			st := styles.DialogOptionInactive
			sel := isSelected != nil && isSelected(m.Path)
			switch {
			case i == state.PathCursor && sel:
				st = styles.DialogOptionActiveSelected
			case i == state.PathCursor:
				st = styles.DialogOptionActive
			case sel:
				st = styles.DialogOptionSelected
			}
			for c := m.Start; c < m.End && c < len(cells); c++ {
				cells[c].St = styles.DialogTextSelectStyle(st)
			}
			if m.End >= len(cells) {
				// DrawCells pads with the last cell's style; keep the highlight on the text only.
				cells = append(cells, previewpanel.AnsiCell{R: ' ', St: lineStyle})
			}
		}
		previewpanel.DrawCells(screen, contentX, y, contentW, cells, lineStyle)
	}
	draw.DrawDialogListScrollbar(screen, rect, rect.Y+1, listH, len(state.Lines), scroll, scrollbarStyle, borderStyle, styles)

	// separator at buttonY-2, blank row at buttonY-1 (DrawDialogFrame fills surface), button at buttonY
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-2, borderStyle)
	buttons := []draw.DialogButtonSpec{{Label: "OK", Shortcut: 'O', Focused: true}}
	if state.Running {
		buttons = []draw.DialogButtonSpec{
			{Label: "Background", Shortcut: 'B', Focused: state.Focus == 0},
			{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == 1, Destructive: true},
		}
	}
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, buttons, styles)
}
