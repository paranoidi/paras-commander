package dialog

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
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
	// outputBody is the reserved region the output lines are painted into (ANSI colored cells
	// with path highlights and a scrollbar are bespoke, so it is an overlay, not a dlg list).
	outputBody = dlg.ID("output")
)

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

// CommandOutputDialogView is the output dialog's widget spec: a reserved output region above the
// OK button (finished) or the Background / Cancel buttons (running; the title carries the
// Running / Queued status). ok is false when the dialog does not fit the layout.
func CommandOutputDialogView(layout Layout, state CommandOutputDialogState, styles theme.Theme) (spec dlg.Spec, listH int, ok bool) {
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
	return dlg.Spec{
		Title: title, Width: prefW, Unclamped: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{{{ID: outputBody, Kind: dlg.KindSpacer, Height: listH}}}}},
		Buttons: commandOutputButtons(state.Running),
	}, listH, ok
}

// commandOutputButtons is the button strip: OK when finished, Background / Cancel while running.
func commandOutputButtons(running bool) []dlg.Button {
	if running {
		return []dlg.Button{
			{ID: OutputBackground, Label: "Background", Mnemonic: 'B'},
			{ID: OutputCancel, Label: "Cancel", Mnemonic: 'C', Destructive: true},
		}
	}
	return []dlg.Button{{ID: dlg.OK, Label: "OK", Mnemonic: 'O'}}
}

// CommandOutputDialogKeys is the spec for key handling alone: it carries the buttons but needs no
// layout, since the output region is not focusable.
func CommandOutputDialogKeys(state CommandOutputDialogState) dlg.Spec {
	return dlg.Spec{Buttons: commandOutputButtons(state.Running)}
}

// ComputeCommandOutputDialogMetrics calculates dialog geometry from layout and state.
func ComputeCommandOutputDialogMetrics(layout Layout, state CommandOutputDialogState) (m CommandOutputDialogMetrics, ok bool) {
	spec, listH, fits := CommandOutputDialogView(layout, state, theme.Theme{})
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

// DrawCommandOutputDialog renders the command output modal overlay: the dlg frame, separator and
// buttons, then the output lines painted into the reserved output region.
func DrawCommandOutputDialog(screen tcell.Screen, layout Layout, state CommandOutputDialogState, styles theme.Theme, scrollbarStyle uiscrollbar.Style, isSelected func(string) bool) {
	if !state.Open {
		return
	}
	spec, listH, ok := CommandOutputDialogView(layout, state, styles)
	if !ok {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
	p, placed := dlg.Place(layout.Width, layout.Height, spec)
	if !placed {
		return
	}
	rect := p.Rect
	contentW := draw.DialogContentWidth(rect)
	_, dbg, _ := styles.DialogSurface.Decompose()
	borderStyle := styles.DialogFrame.Background(dbg)
	lineStyle := styles.DialogText.Background(dbg)

	contentX := draw.DialogTextX(rect)
	scroll := state.Scroll
	if state.Follow {
		scroll = max(0, len(state.Lines)-listH)
	}
	// Paths are ordered by Line: walk them alongside the visible rows.
	pi := sort.Search(len(state.Paths), func(i int) bool { return state.Paths[i].Line >= scroll })
	for row := range listH {
		y := rect.Y + 1 + row
		line := ""
		if idx := scroll + row; idx < len(state.Lines) {
			line = state.Lines[idx]
		}
		// ponytail: per-line SGR parse; a color left open across \n does not carry to the next
		// line. Parse the whole output once into cell rows if a tool needs that.
		cells := previewpanel.AnsiStyledCells(line, lineStyle)
		for ; pi < len(state.Paths) && state.Paths[pi].Line == scroll+row; pi++ {
			i, m := pi, state.Paths[pi]
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
}
