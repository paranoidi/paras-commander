package dialog

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func DrawDebounceCalibrateDialog(screen tcell.Screen, layout Layout, state DebounceCalibrateDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}

	const (
		width    = 58
		minWidth = 44
		// Fixed for every phase and both status states, so the box never resizes underneath the
		// user — not when Calibrate starts or ends, and not when a status line appears (e.g.
		// "Released too soon" while measuring). The status row is reserved whether or not it is
		// painted.
		height = 14
	)

	status := strings.TrimSpace(state.Status)
	rect, ok := draw.ClampCenteredDialogRect(layout, width, height, minWidth, height)
	if !ok {
		return
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, "Calibrate Debounce", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)

	y := rect.Y + 1
	if state.Phase == DebounceCalibrateMeasuring {
		drawDebounceCalibrateMeasuringBody(screen, rect, textX, textW, y, state, status, styles, dbg)
		return
	}

	primitive.Text(screen, textX, y, textW, "Debounce (ms):", styles.DialogText.Background(dbg))
	y += 2
	draw.DrawSimpleDialogInput(screen, textX, y, textW, state.Value, state.Focus == 0, false, styles)
	y++
	hint := "Used for file-list scroll, quick view, carousel etc."
	primitive.Text(screen, textX, y, textW, hint, styles.DialogText.Background(dbg))
	y++
	// Reserved whether painted or not (see the fixed height above). Sits under the first field
	// because that is the one Calibrate measures and reports on.
	if status != "" {
		primitive.Text(screen, textX, y, textW, status, styles.DialogText.Background(dbg))
	}
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	primitive.Text(screen, textX, y, textW, "Image preview debounce (ms):", styles.DialogText.Background(dbg))
	y += 2
	draw.DrawSimpleDialogInput(screen, textX, y, textW, state.ImageValue, state.Focus == 1, false, styles)

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-2, borderStyle)
	form := NewDialogTrailingButtonsForm(2, 3)
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.Focus == form.OKIndex()},
		{Label: "Calibrate", Shortcut: 'L', Focused: state.Focus == form.MiddleButtonIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == form.CancelIndex()},
	}, styles)
}

func drawDebounceCalibrateMeasuringBody(screen tcell.Screen, rect draw.Rect, textX, textW, y int, state DebounceCalibrateDialogState, status string, styles theme.Theme, dbg tcell.Color) {
	required := MeasureMinRepeatSamples()
	// Sampling keeps running while the key is held, so cap the readout at the target: a full bar
	// next to "17/8" reads as a bug, and the wider label shifts the centered row as it grows.
	collected := min(len(state.Samples), required)
	line := "Hold a letter or arrow key."
	if state.MeasureStep == MeasureCollecting {
		line = "Keep holding" + string(primitive.Ellipsis)
	}
	primitive.Text(screen, textX, y, textW, line, styles.DialogText.Background(dbg))
	y += 2
	const progressBarCells = 24
	bar := CalibrationProgressBar(progressBarCells, collected, required)
	label := fmt.Sprintf("%s %d/%d", bar, collected, required)
	drawDebounceCalibrateCenteredRow(screen, rect, y, label, styles.DialogText.Background(dbg))
	y += 2
	if status != "" {
		drawDebounceCalibrateCenteredRow(screen, rect, y, status, styles.DialogText.Background(dbg))
	}
}

func drawDebounceCalibrateCenteredRow(screen tcell.Screen, rect draw.Rect, y int, text string, style tcell.Style) {
	if text == "" {
		return
	}
	innerW := draw.DialogContentWidth(rect)
	x := draw.DialogTextX(rect)
	n := utf8.RuneCountInString(text)
	if n > innerW {
		primitive.Text(screen, x, y, innerW, text, style)
		return
	}
	pad := (innerW - n) / 2
	primitive.Text(screen, x+pad, y, innerW-pad, text, style)
}
