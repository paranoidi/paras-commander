package dialog

import "time"

// DebounceCalibrateDialogState is the Options calibrate-debounce dialog.
type DebounceCalibrateDialogState struct {
	Open bool

	Phase  DebounceCalibratePhase
	Focus  int
	Value  string
	Cursor int

	// Preview debounce (focus 1); not calibrated.
	PreviewValue  string
	PreviewCursor int

	Status string

	// Measurement sub-flow (Phase == DebounceCalibrateMeasuring).
	MeasureStep   MeasureStep
	Samples       []int64
	PressKey      string
	LastEventAt   time.Time
	EventCount    int
	InputSnapshot string // Value before Calibrate; restored on Esc during measuring
}
