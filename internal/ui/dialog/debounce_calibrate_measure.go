package dialog

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/config"
)

// DebounceCalibratePhase is the calibrate dialog UI phase.
type DebounceCalibratePhase int

const (
	DebounceCalibrateEdit DebounceCalibratePhase = iota
	DebounceCalibrateMeasuring
)

// MeasureStep is the in-hold measurement state.
type MeasureStep int

const (
	MeasureAwaitPress MeasureStep = iota
	MeasureCollecting
)

// KeyFingerprint returns a stable key id for repeat detection, or false when the key is not usable.
func KeyFingerprint(ev *tcell.EventKey) (string, bool) {
	mod := ev.Modifiers()
	if mod != tcell.ModNone && mod != tcell.ModShift {
		return "", false
	}
	switch ev.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight:
		return fmt.Sprintf("key:%d", ev.Key()), true
	case tcell.KeyRune:
		if !unicode.IsPrint(ev.Rune()) {
			return "", false
		}
		return fmt.Sprintf("rune:%q", ev.Rune()), true
	default:
		return "", false
	}
}

// ValidCalibrationRepeatMS reports whether a repeat interval is plausible.
func ValidCalibrationRepeatMS(ms int64) bool {
	return ms >= config.DebounceCalibrationMinRepeatMS && ms <= config.DebounceCalibrationMaxRepeatMS
}

// ValidCalibrationDelayMS reports whether a press-to-first-repeat delay is plausible.
func ValidCalibrationDelayMS(ms int64) bool {
	return ms >= config.DebounceCalibrationMinDelayMS && ms <= config.DebounceCalibrationMaxDelayMS
}

// RepeatCalibrationHold tracks one continuous hold sample stream.
type RepeatCalibrationHold struct {
	PressKey    string
	LastEventAt time.Time
	EventCount  int
	Delay       int64 // press-to-first-repeat interval; 0 = not captured yet
	Samples     []int64
}

// RecordRepeatCalibrationEvent ingests one key event while sampling repeat speed. The interval from
// initial press to first repeat (EventCount reaches 2) is the delay sample; later intervals are
// repeat-speed samples.
func RecordRepeatCalibrationEvent(h RepeatCalibrationHold, fp string, now time.Time) RepeatCalibrationHold {
	if h.PressKey == "" {
		h.PressKey = fp
		h.LastEventAt = now
		h.EventCount = 1
		return h
	}
	if fp != h.PressKey {
		return h
	}
	delta := now.Sub(h.LastEventAt).Milliseconds()
	h.LastEventAt = now
	h.EventCount++
	switch {
	case h.EventCount == 2:
		if ValidCalibrationDelayMS(delta) {
			h.Delay = delta
		}
	case h.EventCount >= 3 && ValidCalibrationRepeatMS(delta):
		h.Samples = append(h.Samples, delta)
	}
	return h
}

// RepeatCalibrationReleaseReady reports whether a hold captured a delay sample and enough repeat
// intervals to finish.
func RepeatCalibrationReleaseReady(delay int64, samples []int64) bool {
	return delay > 0 && len(samples) >= MeasureMinRepeatSamples()
}

// AverageRepeatIntervalMS rounds the arithmetic mean of repeat intervals.
func AverageRepeatIntervalMS(samples []int64) int64 {
	if len(samples) == 0 {
		return 0
	}
	var sum int64
	for _, s := range samples {
		sum += s
	}
	avg := float64(sum) / float64(len(samples))
	return int64(avg + 0.5)
}

// RecommendedDebounceMS adds the calibration margin and clamps to config bounds.
func RecommendedDebounceMS(avgMS int64, marginMS int) int {
	ms := int(avgMS) + marginMS
	return ClampDebounceMS(ms)
}

// MaxCalibrationDelayMS returns the largest delay sample, or 0 when none.
func MaxCalibrationDelayMS(delays []int64) int64 {
	var max int64
	for _, d := range delays {
		if d > max {
			max = d
		}
	}
	return max
}

// RecommendedMediaPreviewDebounceMS derives the preview debounce from the worst-case (max, not
// average) measured delay, since the debounce must outlast the delay on every hold.
func RecommendedMediaPreviewDebounceMS(delays []int64) int {
	return ClampDebounceMS(int(MaxCalibrationDelayMS(delays)) + config.DebounceCalibrationMediaPreviewMarginMS)
}

// ClampDebounceMS clamps to 0..KeyRepeatDebounceMaxMS.
func ClampDebounceMS(ms int) int {
	if ms < 0 {
		return 0
	}
	if ms > config.KeyRepeatDebounceMaxMS {
		return config.KeyRepeatDebounceMaxMS
	}
	return ms
}

// ParseDebounceMSInput parses a non-negative integer debounce field.
func ParseDebounceMSInput(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n < 0 || n > config.KeyRepeatDebounceMaxMS {
		return 0, fmt.Errorf("out of range")
	}
	return n, nil
}

// FormatDebounceMS formats a debounce value for the dialog input.
func FormatDebounceMS(ms int) string {
	return strconv.Itoa(ms)
}

// MeasureMinRepeatSamples returns how many repeat intervals one hold must collect.
func MeasureMinRepeatSamples() int {
	return config.DebounceCalibrationMinRepeatSamples
}

// MeasureReleaseIdle returns idle duration used to infer key release once repeats are flowing.
func MeasureReleaseIdle() time.Duration {
	return time.Duration(config.DebounceCalibrationReleaseIdleMS) * time.Millisecond
}

// MeasureFirstRepeatWait returns how long to wait after the initial press for the first repeat,
// before an idle timeout would otherwise be mistaken for a release.
func MeasureFirstRepeatWait() time.Duration {
	return time.Duration(config.DebounceCalibrationMaxDelayMS) * time.Millisecond
}

// MeasureHolds returns how many press-and-hold rounds Calibrate Debounce runs.
func MeasureHolds() int {
	return config.DebounceCalibrationHolds
}

// CalibrationMarginMS returns the margin added to the averaged repeat interval.
func CalibrationMarginMS() int {
	return config.DebounceCalibrationMarginMS
}

// CalibrationMediaPreviewMarginMS returns the margin added to the max measured delay.
func CalibrationMediaPreviewMarginMS() int {
	return config.DebounceCalibrationMediaPreviewMarginMS
}

// CalibrationProgressBar renders a ████░░░░ bar for collected samples (no frame icons).
func CalibrationProgressBar(width, collected, required int) string {
	if width < 1 {
		width = 1
	}
	if required <= 0 {
		required = 1
	}
	filled := (collected * width) / required
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
