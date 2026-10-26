package app

import (
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func (a *App) openDebounceCalibrateDialog() {
	a.clearTransientMessage()
	value := dialog.FormatDebounceMS(a.config.UI.KeyRepeatDebounceMS)
	previewValue := dialog.FormatDebounceMS(a.config.UI.MediaPreviewDebounceMS)
	a.model.DebounceCalibrateDialog = dialog.DebounceCalibrateDialogState{
		Open:          true,
		Phase:         dialog.DebounceCalibrateEdit,
		Focus:         dialog.DebounceDelay,
		Value:         value,
		Cursor:        utf8.RuneCountInString(value),
		PreviewValue:  previewValue,
		PreviewCursor: utf8.RuneCountInString(previewValue),
	}
}

func (a *App) closeDebounceCalibrateDialog() {
	a.clearDebounceCalibrateReleaseTimer()
	a.model.DebounceCalibrateDialog = dialog.DebounceCalibrateDialogState{}
}

func (a *App) applyDebounceCalibrateDialog() {
	st := &a.model.DebounceCalibrateDialog
	ms, err := dialog.ParseDebounceMSInput(st.Value)
	if err != nil {
		st.Focus = dialog.DebounceDelay
		st.Status = fmt.Sprintf("Enter 0–%d", config.KeyRepeatDebounceMaxMS)
		return
	}
	previewMS, err := dialog.ParseDebounceMSInput(st.PreviewValue)
	if err != nil {
		st.Focus = dialog.DebouncePreview
		// The status row sits under the first field, so name the field this one is about.
		st.Status = fmt.Sprintf("Media preview: enter 0–%d", config.KeyRepeatDebounceMaxMS)
		return
	}
	a.config.UI.KeyRepeatDebounceMS = ms
	a.config.UI.MediaPreviewDebounceMS = previewMS
	a.closeDebounceCalibrateDialog()
	msg := fmt.Sprintf("Debounce set to %d ms (media previews %d ms)", ms, previewMS)
	patch := map[string]interface{}{
		"ui": map[string]interface{}{
			"key_repeat_debounce_ms":    ms,
			"media_preview_debounce_ms": previewMS,
		},
	}
	if err := a.persistPartial(patch); err != nil {
		msg = fmt.Sprintf("%s (could not write config: %v)", msg, err)
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}

func (a *App) beginDebounceCalibrateMeasuring() {
	st := &a.model.DebounceCalibrateDialog
	st.InputSnapshot = st.Value
	st.PreviewSnapshot = st.PreviewValue
	st.Phase = dialog.DebounceCalibrateMeasuring
	st.MeasureStep = dialog.MeasureAwaitPress
	st.HoldIndex = 0
	st.Samples = nil
	st.Delays = nil
	st.HoldSamples = 0
	st.HoldDelay = 0
	st.PressKey = ""
	st.EventCount = 0
	st.Status = ""
	a.clearDebounceCalibrateReleaseTimer()
}

func (a *App) abortDebounceCalibrateMeasuring() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	st.Phase = dialog.DebounceCalibrateEdit
	st.Value = st.InputSnapshot
	st.Cursor = utf8.RuneCountInString(st.Value)
	st.PreviewValue = st.PreviewSnapshot
	st.PreviewCursor = utf8.RuneCountInString(st.PreviewValue)
	st.Focus = dialog.DebounceCalibrate
	st.Status = ""
	st.MeasureStep = dialog.MeasureAwaitPress
	st.HoldIndex = 0
	st.Samples = nil
	st.Delays = nil
	st.HoldSamples = 0
	st.HoldDelay = 0
	st.PressKey = ""
	st.EventCount = 0
}

// completeDebounceCalibrateHold commits the current hold's delay sample and either starts the next
// hold or, once all holds are done, finishes measuring.
func (a *App) completeDebounceCalibrateHold() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	st.Delays = append(st.Delays, st.HoldDelay)
	st.HoldIndex++
	if st.HoldIndex >= dialog.MeasureHolds() {
		a.finishDebounceCalibrateMeasuring()
		return
	}
	st.HoldSamples = len(st.Samples)
	st.HoldDelay = 0
	st.PressKey = ""
	st.EventCount = 0
	st.MeasureStep = dialog.MeasureAwaitPress
	st.Status = ""
}

func (a *App) finishDebounceCalibrateMeasuring() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	avg := dialog.AverageRepeatIntervalMS(st.Samples)
	ms := dialog.RecommendedDebounceMS(avg, dialog.CalibrationMarginMS())
	maxDelay := dialog.MaxCalibrationDelayMS(st.Delays)
	previewMS := dialog.RecommendedMediaPreviewDebounceMS(st.Delays)
	st.Phase = dialog.DebounceCalibrateEdit
	st.Value = dialog.FormatDebounceMS(ms)
	st.Cursor = utf8.RuneCountInString(st.Value)
	st.PreviewValue = dialog.FormatDebounceMS(previewMS)
	st.PreviewCursor = utf8.RuneCountInString(st.PreviewValue)
	st.Focus = dialog.DebounceDelay
	st.Status = fmt.Sprintf("Repeat %d ms, delay %d ms (margins %d/%d ms).",
		avg, maxDelay, dialog.CalibrationMarginMS(), dialog.CalibrationMediaPreviewMarginMS())
	st.MeasureStep = dialog.MeasureAwaitPress
	st.HoldIndex = 0
	st.Samples = nil
	st.Delays = nil
	st.HoldSamples = 0
	st.HoldDelay = 0
	st.PressKey = ""
	st.EventCount = 0
}

func (a *App) failDebounceCalibrateMeasuringTooSoon() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	st.Samples = st.Samples[:st.HoldSamples]
	st.HoldDelay = 0
	st.PressKey = ""
	st.EventCount = 0
	st.MeasureStep = dialog.MeasureAwaitPress
	st.Status = fmt.Sprintf("Released too soon; hold until %d/%d on the bar.", 0, dialog.MeasureMinRepeatSamples())
}

func (a *App) clearDebounceCalibrateReleaseTimer() {
	a.debounceCalibrateRelease.Stop()
}

func (a *App) armDebounceCalibrateReleaseTimer() {
	a.armDebounceCalibrateReleaseTimerFor(dialog.MeasureReleaseIdle())
}

// armDebounceCalibrateReleaseTimerFor arms the release-inference timer with an explicit duration:
// the longer MeasureFirstRepeatWait right after the initial press (waiting for the first repeat,
// which can itself take up to the max delay), the shorter MeasureReleaseIdle once repeats flow.
func (a *App) armDebounceCalibrateReleaseTimerFor(d time.Duration) {
	a.debounceCalibrateRelease.Arm(d, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(debounceCalibrateReleasePayload{}))
	})
}

type debounceCalibrateReleasePayload struct{}

func (a *App) applyDebounceCalibrateReleasePayload() bool {
	st := &a.model.DebounceCalibrateDialog
	if !st.Open || st.Phase != dialog.DebounceCalibrateMeasuring || st.MeasureStep != dialog.MeasureCollecting {
		return false
	}
	if dialog.RepeatCalibrationReleaseReady(st.HoldDelay, st.Samples[st.HoldSamples:]) {
		a.completeDebounceCalibrateHold()
		return true
	}
	a.failDebounceCalibrateMeasuringTooSoon()
	return true
}

func (a *App) handleDebounceCalibrateDialogKey(event *tcell.EventKey) {
	st := &a.model.DebounceCalibrateDialog
	if st.Phase == dialog.DebounceCalibrateMeasuring {
		a.handleDebounceCalibrateMeasuringKey(event)
		return
	}

	view := dialog.DebounceCalibrateView(*st)
	if ev, ok := dlg.HandleMnemonic(view, &st.Focus, event); ok {
		a.applyDebounceCalibrateEvent(ev) // Calibrate (Alt+L) wins over the inputs' editing
		return
	}
	onInput := st.Focus == dialog.DebounceDelay || st.Focus == dialog.DebouncePreview
	if onInput && event.Key() != tcell.KeyEnter {
		st.Status = ""
	}
	if ev, ok := dlg.HandleKey(view, &st.Focus, event); ok {
		a.applyDebounceCalibrateEvent(ev)
		return
	}
	switch st.Focus {
	case dialog.DebounceDelay:
		handleDebounceCalibrateInputKey(event, &st.Value, &st.Cursor)
	case dialog.DebouncePreview:
		handleDebounceCalibrateInputKey(event, &st.PreviewValue, &st.PreviewCursor)
	}
}

// applyDebounceCalibrateEvent acts on a widget event of the calibration dialog (key or click).
func (a *App) applyDebounceCalibrateEvent(ev dlg.Event) {
	st := &a.model.DebounceCalibrateDialog
	switch {
	case ev.Kind == dlg.EventCaret && ev.ID == dialog.DebounceDelay:
		st.Cursor = ev.Index
	case ev.Kind == dlg.EventCaret:
		st.PreviewCursor = ev.Index
	case ev.ID == "":
	case ev.ID == dlg.Cancel:
		a.closeDebounceCalibrateDialog()
	case ev.ID == dialog.DebounceCalibrate:
		a.beginDebounceCalibrateMeasuring()
	default: // OK, or Enter in an input
		a.applyDebounceCalibrateDialog()
	}
}

// handleDebounceCalibrateInputKey edits one numeric field of the dialog; shared by both inputs.
// Keys it does not know are ignored, and non-digit runes are swallowed.
func handleDebounceCalibrateInputKey(event *tcell.EventKey, value *string, cursor *int) {
	switch event.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if *cursor > 0 {
			runes := []rune(*value)
			*value = string(runes[:*cursor-1]) + string(runes[*cursor:])
			*cursor--
		}
	case tcell.KeyDelete:
		runes := []rune(*value)
		if *cursor < len(runes) {
			*value = string(runes[:*cursor]) + string(runes[*cursor+1:])
		}
	case tcell.KeyLeft:
		if *cursor > 0 {
			*cursor--
		}
	case tcell.KeyRight:
		if *cursor < utf8.RuneCountInString(*value) {
			*cursor++
		}
	case tcell.KeyHome:
		*cursor = 0
	case tcell.KeyEnd:
		*cursor = utf8.RuneCountInString(*value)
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone || !unicode.IsDigit(event.Rune()) {
			return
		}
		runes := []rune(*value)
		runes = append(runes[:*cursor], append([]rune{event.Rune()}, runes[*cursor:]...)...)
		*value = string(runes)
		*cursor++
	}
}

func (a *App) handleDebounceCalibrateMeasuringKey(event *tcell.EventKey) {
	st := &a.model.DebounceCalibrateDialog
	fp, ok := dialog.KeyFingerprint(event)
	if !ok {
		return
	}
	now := time.Now()
	switch st.MeasureStep {
	case dialog.MeasureAwaitPress:
		st.PressKey = fp
		st.LastEventAt = now
		st.EventCount = 1
		st.MeasureStep = dialog.MeasureCollecting
		st.Status = ""
		a.armDebounceCalibrateReleaseTimerFor(dialog.MeasureFirstRepeatWait())
	case dialog.MeasureCollecting:
		if fp != st.PressKey {
			return
		}
		hold := dialog.RecordRepeatCalibrationEvent(dialog.RepeatCalibrationHold{
			PressKey:    st.PressKey,
			LastEventAt: st.LastEventAt,
			EventCount:  st.EventCount,
			Delay:       st.HoldDelay,
			Samples:     st.Samples,
		}, fp, now)
		st.PressKey = hold.PressKey
		st.LastEventAt = hold.LastEventAt
		st.EventCount = hold.EventCount
		st.HoldDelay = hold.Delay
		st.Samples = hold.Samples
		a.clearDebounceCalibrateReleaseTimer()
		a.armDebounceCalibrateReleaseTimer()
	}
}
