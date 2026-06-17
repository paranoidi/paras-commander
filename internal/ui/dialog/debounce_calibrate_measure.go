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

// ValidCalibrationSampleMS is an alias kept for tests referencing the old name.
func ValidCalibrationSampleMS(ms int64) bool {
	return ValidCalibrationRepeatMS(ms)
}

// RepeatCalibrationHold tracks one continuous hold sample stream.
type RepeatCalibrationHold struct {
	PressKey    string
	LastEventAt time.Time
	EventCount  int
	Samples     []int64
}

// RecordRepeatCalibrationEvent ingests one key event while sampling repeat speed.
// The interval from initial press to first repeat is ignored; later intervals are repeat-speed samples.
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
	if h.EventCount >= 3 && ValidCalibrationRepeatMS(delta) {
		h.Samples = append(h.Samples, delta)
	}
	return h
}

// RepeatCalibrationReleaseReady reports whether enough repeat intervals were collected.
func RepeatCalibrationReleaseReady(samples []int64) bool {
	return len(samples) >= MeasureMinRepeatSamples()
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

// MeasureReleaseIdle returns idle duration used to infer key release.
func MeasureReleaseIdle() time.Duration {
	return time.Duration(config.DebounceCalibrationReleaseIdleMS) * time.Millisecond
}

// CalibrationMarginMS returns the hardcoded margin added after measurement.
func CalibrationMarginMS() int {
	return config.DebounceCalibrationMarginMS
}

// CalibrationProgressBar renders a ████░░░░ bar for collected samples (no frame glyphs).
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
