package dialog

import "time"

// DebounceCalibrateDialogState is the Options calibrate-debounce dialog.
type DebounceCalibrateDialogState struct {
	Open bool

	Phase  DebounceCalibratePhase
	Focus  int
	Value  string
	Cursor int

	// Preview debounce (focus 1); derived from the largest measured key-repeat delay when calibrated.
	PreviewValue  string
	PreviewCursor int

	Status string

	// Measurement sub-flow (Phase == DebounceCalibrateMeasuring), run over MeasureHolds() separate
	// press-and-hold rounds. Samples/Delays accumulate across holds that finished successfully;
	// HoldSamples is the index into Samples where the in-progress hold's own samples begin, so a
	// hold that releases too soon can be trimmed back to it without losing earlier holds.
	MeasureStep     MeasureStep
	HoldIndex       int
	Samples         []int64
	Delays          []int64
	HoldSamples     int
	HoldDelay       int64
	PressKey        string
	LastEventAt     time.Time
	EventCount      int
	InputSnapshot   string // Value before Calibrate; restored on Esc during measuring
	PreviewSnapshot string // PreviewValue before Calibrate; restored on Esc during measuring
}
