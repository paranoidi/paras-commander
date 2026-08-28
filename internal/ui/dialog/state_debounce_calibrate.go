package dialog

import "time"

// DebounceCalibrateDialogState is the Options calibrate-debounce dialog.
type DebounceCalibrateDialogState struct {
	Open bool

	Phase  DebounceCalibratePhase
	Focus  int
	Value  string
	Cursor int

	// Image/media preview debounce (focus 1); not calibrated.
	ImageValue  string
	ImageCursor int

	Status string

	// Measurement sub-flow (Phase == DebounceCalibrateMeasuring).
	MeasureStep   MeasureStep
	Samples       []int64
	PressKey      string
	LastEventAt   time.Time
	EventCount    int
	InputSnapshot string // Value before Calibrate; restored on Esc during measuring
}
