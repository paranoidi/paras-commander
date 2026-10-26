package dialog

import (
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Calibrate-debounce widget IDs. The OK and Cancel buttons are dlg.OK and dlg.Cancel.
const (
	DebounceDelay     = dlg.ID("delay")
	DebouncePreview   = dlg.ID("preview")
	DebounceCalibrate = dlg.ID("calibrate")
)

// debounceCalibrateHeight is the dialog height of the edit phase: it also fixes the measuring
// phase, so the box never resizes when Calibrate starts or ends or a status line appears.
const debounceCalibrateHeight = 12

// DebounceCalibrateView is the calibrate dialog's widget spec. The edit phase is the two numeric
// inputs with their hints (the reserved status row sits under the first one, because that is the
// field Calibrate measures and reports on) above OK / Calibrate / Cancel; the measuring phase is
// text only: the prompt, the progress bar and the status line.
func DebounceCalibrateView(s DebounceCalibrateDialogState) dlg.Spec {
	status := strings.TrimSpace(s.Status)
	spec := dlg.Spec{Title: "Calibrate Debounce", Width: 58, MinWidth: 44, MinHeight: debounceCalibrateHeight}
	if s.Phase == DebounceCalibrateMeasuring {
		required := MeasureMinRepeatSamples()
		// Sampling keeps running while the key is held, so cap the readout at the target: a full bar
		// next to "17/8" reads as a bug, and the wider label shifts the centered row as it grows.
		collected := min(len(s.Samples[s.HoldSamples:]), required)
		progress := fmt.Sprintf("(%d/%d)", s.HoldIndex+1, MeasureHolds())
		line := "Hold a letter or arrow key " + progress + "."
		if s.MeasureStep == MeasureCollecting {
			line = "Keep holding" + string(primitive.Ellipsis) + " " + progress
		}
		bar := fmt.Sprintf("%s %d/%d", CalibrationProgressBar(24, collected, required), collected, required)
		spec.Groups = []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: line},
			{Kind: dlg.KindLabel},
			{Kind: dlg.KindLabel, Label: bar, Center: true},
			{Kind: dlg.KindLabel},
			{Kind: dlg.KindLabel, Label: status, Center: true},
		}}}}
		return spec
	}
	spec.Groups = []dlg.Group{{Columns: [][]dlg.Item{{
		{Kind: dlg.KindLabel, Label: "Debounce (ms):"},
		{ID: DebounceDelay, Kind: dlg.KindInput, Simple: true, Value: s.Value, Cursor: s.Cursor},
		{Kind: dlg.KindLabel, Label: "Used for file-list scroll and directory listings."},
		{Kind: dlg.KindLabel, Label: status},
		{Kind: dlg.KindSeparator},
		{Kind: dlg.KindLabel, Label: "Media preview debounce (ms):"},
		{ID: DebouncePreview, Kind: dlg.KindInput, Simple: true, Value: s.PreviewValue, Cursor: s.PreviewCursor},
		{Kind: dlg.KindLabel, Label: "Used for images, video/audio and preview command rules."},
	}}}}
	spec.Buttons = []dlg.Button{
		dlg.OKButton,
		{ID: DebounceCalibrate, Label: "Calibrate", Mnemonic: 'L'},
		dlg.CancelButton,
	}
	return spec
}
