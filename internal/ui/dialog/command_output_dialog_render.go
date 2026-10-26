package dialog

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Command output dialog widget IDs. The finished state has the conventional dlg.OK button; while
// the run is live the buttons are Background (keeps it running) and Cancel (kills it).
const (
	OutputBackground = dlg.ID("background")
	OutputCancel     = dlg.ID("cancel")
	// outputBody is the ScrollOnly list of output lines.
	outputBody = dlg.ID("output")
)

// CommandOutputViewContext is what CommandOutputDialogView needs besides the layout and state.
type CommandOutputViewContext struct {
	Styles         theme.Theme
	ScrollbarStyle uiscrollbar.Style
	// IsSelected reports whether a detected path is selected in the active panel (nil: none is).
	IsSelected func(path string) bool
}

// CommandOutputDialogMetrics holds computed geometry for the output dialog.
type CommandOutputDialogMetrics struct {
	Rect         draw.Rect
	ListH        int
	ContentWidth int
}

// cmdOutputOverhead: top border/title + separator + button + bottom border.
const cmdOutputOverhead = 4

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

// commandOutputSize returns the preferred outer width and the visible output row count for the
// layout; ok is false when the dialog does not fit.
func commandOutputSize(layout Layout, state CommandOutputDialogState) (prefW, listH int, ok bool) {
	const minListH = 4
	const minW = 40

	prefW = ParseDialogDimension(state.PrefWidth, layout.Width)
	if prefW == 0 {
		prefW = layout.Width * 4 / 5
	}
	if prefW < minW {
		prefW = minW
	}
	maxW := layout.Width - 4
	if maxW < minW {
		return 0, 0, false
	}
	if prefW > maxW {
		prefW = maxW
	}

	prefH := ParseDialogDimension(state.PrefHeight, layout.Height)
	if prefH == 0 {
		prefH = layout.Height * 3 / 5
	}
	listH = max(prefH-cmdOutputOverhead, minListH)
	if height := listH + cmdOutputOverhead; height > layout.Height-2 {
		listH = layout.Height - 2 - cmdOutputOverhead
		if listH < minListH {
			return 0, 0, false
		}
	}
	return prefW, listH, true
}

// CommandOutputDialogView is the output dialog's widget spec: the output lines, a ScrollOnly list
// (following the end while Follow is set), above the OK button (finished) or the Background /
// Cancel buttons (running; the title carries the Running / Queued status). ok is false when the
// dialog does not fit the layout.
func CommandOutputDialogView(layout Layout, state CommandOutputDialogState, ctx CommandOutputViewContext) (spec dlg.Spec, listH int, ok bool) {
	styles := ctx.Styles
	prefW, listH, ok := commandOutputSize(layout, state)
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
	scroll := state.Scroll
	if state.Follow {
		scroll = len(state.Lines) // clamped to the last page
	}
	_, bg, _ := styles.DialogSurface.Decompose()
	lineStyle := styles.DialogText.Background(bg)
	body := scrollList(outputBody, len(state.Lines), listH, scroll, func(i int) dlg.ListRow {
		return outputRow(state, i, lineStyle, styles, ctx.IsSelected)
	})
	body.Fit = dlg.FitNone
	return dlg.Spec{
		Title: title, Width: prefW, Unclamped: true, ScrollbarStyle: ctx.ScrollbarStyle,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{{body}}}},
		Buttons: commandOutputButtons(state.Running),
	}, listH, ok
}

// outputRow is output line i as a list row: its ANSI colors and the detected path highlights
// (cursor and selection states) as spans over the plain text.
func outputRow(state CommandOutputDialogState, i int, lineStyle tcell.Style, styles theme.Theme, isSelected func(string) bool) dlg.ListRow {
	// ponytail: per-line SGR parse; a color left open across \n does not carry to the next line.
	// Parse the whole output once into cell rows if a tool needs that.
	cells := previewpanel.AnsiStyledCells(state.Lines[i], lineStyle)
	for pi := sort.Search(len(state.Paths), func(k int) bool { return state.Paths[k].Line >= i }); pi < len(state.Paths) && state.Paths[pi].Line == i; pi++ {
		m := state.Paths[pi]
		st := styles.DialogOptionInactive
		sel := isSelected != nil && isSelected(m.Path)
		switch {
		case pi == state.PathCursor && sel:
			st = styles.DialogOptionActiveSelected
		case pi == state.PathCursor:
			st = styles.DialogOptionActive
		case sel:
			st = styles.DialogOptionSelected
		}
		for c := m.Start; c < m.End && c < len(cells); c++ {
			cells[c].St = styles.DialogTextSelectStyle(st)
		}
	}
	var text strings.Builder
	var spans []primitive.Span
	for k, c := range cells { // one rune per cell, so cell k is rune k of the text
		if c.R < ' ' {
			c.R = ' ' // a tab or other control rune takes one blank cell
		}
		text.WriteRune(c.R)
		if c.St == lineStyle {
			continue
		}
		if n := len(spans); n > 0 && spans[n-1].End == k && spans[n-1].Style == c.St {
			spans[n-1].End++
			continue
		}
		spans = append(spans, primitive.Span{Start: k, End: k + 1, Style: c.St})
	}
	return dlg.ListRow{Text: text.String(), Spans: spans}
}

// commandOutputButtons is the button strip: OK when finished, Background / Cancel while running.
func commandOutputButtons(running bool) []dlg.Button {
	if running {
		return []dlg.Button{
			{ID: OutputBackground, Label: "Background", Mnemonic: 'B'},
			{ID: OutputCancel, Label: "Cancel", Mnemonic: 'C', Destructive: true},
		}
	}
	return []dlg.Button{dlg.OKButton}
}

// ComputeCommandOutputDialogMetrics calculates dialog geometry from layout and state.
func ComputeCommandOutputDialogMetrics(layout Layout, state CommandOutputDialogState) (m CommandOutputDialogMetrics, ok bool) {
	spec, listH, fits := CommandOutputDialogView(layout, state, CommandOutputViewContext{})
	if !fits {
		return m, false
	}
	p, placed := dlg.Place(layout.Width, layout.Height, spec)
	if !placed {
		return m, false
	}
	return CommandOutputDialogMetrics{Rect: p.Rect, ListH: listH, ContentWidth: draw.DialogContentWidth(p.Rect)}, true
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

// DrawCommandOutputDialog renders the command output modal overlay from its widget spec.
func DrawCommandOutputDialog(screen tcell.Screen, layout Layout, state CommandOutputDialogState, styles theme.Theme, scrollbarStyle uiscrollbar.Style, isSelected func(string) bool) {
	if !state.Open {
		return
	}
	spec, _, ok := CommandOutputDialogView(layout, state, CommandOutputViewContext{Styles: styles, ScrollbarStyle: scrollbarStyle, IsSelected: isSelected})
	if !ok {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
}
