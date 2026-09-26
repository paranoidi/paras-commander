package dialog

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/config"
)

func TestKeyFingerprint(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)
	fp, ok := KeyFingerprint(ev)
	if !ok || fp != `rune:'a'` {
		t.Fatalf("KeyFingerprint = %q, %v", fp, ok)
	}
	ev = tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	fp, ok = KeyFingerprint(ev)
	if !ok || fp != fmt.Sprintf("key:%d", tcell.KeyDown) {
		t.Fatalf("arrow KeyFingerprint = %q, %v", fp, ok)
	}
	ev = tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModCtrl)
	if _, ok = KeyFingerprint(ev); ok {
		t.Fatal("ctrl rune should be rejected")
	}
}

func TestValidCalibrationRepeatMS(t *testing.T) {
	if ValidCalibrationRepeatMS(config.DebounceCalibrationMinRepeatMS - 1) {
		t.Fatal("below min should be invalid")
	}
	if !ValidCalibrationRepeatMS(50) {
		t.Fatal("50ms should be valid")
	}
	if ValidCalibrationRepeatMS(config.DebounceCalibrationMaxRepeatMS + 1) {
		t.Fatal("above max should be invalid")
	}
}

func TestRecordRepeatCalibrationEventCapturesDelayThenRepeats(t *testing.T) {
	fp := `rune:'a'`
	t0 := time.Now()
	h := RecordRepeatCalibrationEvent(RepeatCalibrationHold{}, fp, t0)
	if h.EventCount != 1 || h.Delay != 0 || len(h.Samples) != 0 {
		t.Fatalf("press: eventCount=%d delay=%d samples=%d", h.EventCount, h.Delay, len(h.Samples))
	}
	h = RecordRepeatCalibrationEvent(h, fp, t0.Add(400*time.Millisecond))
	if h.EventCount != 2 || h.Delay != 400 || len(h.Samples) != 0 {
		t.Fatalf("first repeat: eventCount=%d delay=%d samples=%d, want delay=400 no repeat sample",
			h.EventCount, h.Delay, len(h.Samples))
	}
	h = RecordRepeatCalibrationEvent(h, fp, t0.Add(450*time.Millisecond))
	if len(h.Samples) != 1 || h.Samples[0] != 50 {
		t.Fatalf("second repeat interval = %v, want [50]", h.Samples)
	}
}

func TestRecordRepeatCalibrationEventRejectsOutOfRangeDelay(t *testing.T) {
	fp := `rune:'a'`
	t0 := time.Now()
	h := RecordRepeatCalibrationEvent(RepeatCalibrationHold{}, fp, t0)
	h = RecordRepeatCalibrationEvent(h, fp, t0.Add(2*time.Second)) // above DebounceCalibrationMaxDelayMS
	if h.Delay != 0 {
		t.Fatalf("out-of-range delay should be rejected, got %d", h.Delay)
	}
}

func TestAverageRepeatIntervalMS(t *testing.T) {
	if got := AverageRepeatIntervalMS([]int64{40, 50, 60}); got != 50 {
		t.Fatalf("avg = %d, want 50", got)
	}
}

func TestRepeatCalibrationReleaseReady(t *testing.T) {
	min := MeasureMinRepeatSamples()
	samples := make([]int64, min-1)
	if RepeatCalibrationReleaseReady(400, samples) {
		t.Fatal("should not be ready below minimum")
	}
	samples = append(samples, 50)
	if RepeatCalibrationReleaseReady(0, samples) {
		t.Fatal("should not be ready without a delay sample")
	}
	if !RepeatCalibrationReleaseReady(400, samples) {
		t.Fatal("should be ready at minimum with a delay sample")
	}
}

func TestRecommendedDebounceMS(t *testing.T) {
	got := RecommendedDebounceMS(40, config.DebounceCalibrationMarginMS)
	if want := 40 + config.DebounceCalibrationMarginMS; got != want {
		t.Fatalf("recommended = %d, want %d", got, want)
	}
	got = RecommendedDebounceMS(20_000, config.DebounceCalibrationMarginMS)
	if got != config.KeyRepeatDebounceMaxMS {
		t.Fatalf("recommended = %d, want clamp %d", got, config.KeyRepeatDebounceMaxMS)
	}
}

func TestRecommendedMediaPreviewDebounceMSUsesMax(t *testing.T) {
	got := RecommendedMediaPreviewDebounceMS([]int64{300, 500, 400})
	want := 500 + config.DebounceCalibrationMediaPreviewMarginMS
	if got != want {
		t.Fatalf("recommended preview = %d, want %d", got, want)
	}
	got = RecommendedMediaPreviewDebounceMS([]int64{20_000})
	if got != config.KeyRepeatDebounceMaxMS {
		t.Fatalf("recommended preview = %d, want clamp %d", got, config.KeyRepeatDebounceMaxMS)
	}
}

func TestCalibrationProgressBar(t *testing.T) {
	bar := CalibrationProgressBar(12, 4, 8)
	if strings.Contains(bar, "[") || strings.Contains(bar, "]") {
		t.Fatalf("progress bar should not include brackets: %q", bar)
	}
	if !strings.Contains(bar, "████") || !strings.Contains(bar, "░░") {
		t.Fatalf("progress bar = %q", bar)
	}
	if utf8.RuneCountInString(bar) != 12 {
		t.Fatalf("progress bar width = %d, want 12", utf8.RuneCountInString(bar))
	}
}

func TestParseDebounceMSInput(t *testing.T) {
	n, err := ParseDebounceMSInput("100")
	if err != nil || n != 100 {
		t.Fatalf("ParseDebounceMSInput(100) = %d, %v", n, err)
	}
	if _, err := ParseDebounceMSInput(""); err == nil {
		t.Fatal("empty should error")
	}
	if _, err := ParseDebounceMSInput("abc"); err == nil {
		t.Fatal("non-numeric should error")
	}
	if _, err := ParseDebounceMSInput("-1"); err == nil {
		t.Fatal("negative should error")
	}
	if _, err := ParseDebounceMSInput("10001"); err == nil {
		t.Fatal("over max should error")
	}
}
