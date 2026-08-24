package app

import (
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) openDebounceCalibrateDialog() {
	a.clearTransientMessage()
	ms := a.config.UI.KeyRepeatDebounceMS
	a.model.DebounceCalibrateDialog = dialog.DebounceCalibrateDialogState{
		Open:   true,
		Phase:  dialog.DebounceCalibrateEdit,
		Focus:  0,
		Value:  dialog.FormatDebounceMS(ms),
		Cursor: utf8.RuneCountInString(dialog.FormatDebounceMS(ms)),
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
		st.Status = fmt.Sprintf("Enter 0–%d", config.KeyRepeatDebounceMaxMS)
		return
	}
	a.config.UI.KeyRepeatDebounceMS = ms
	a.closeDebounceCalibrateDialog()
	msg := fmt.Sprintf("Debounce set to %d ms", ms)
	patch := map[string]interface{}{
		"ui": map[string]interface{}{
			"key_repeat_debounce_ms": ms,
		},
	}
	if err := a.persistPartial(patch); err != nil {
		msg = fmt.Sprintf("Debounce set to %d ms (could not write config: %v)", ms, err)
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}

func (a *App) beginDebounceCalibrateMeasuring() {
	st := &a.model.DebounceCalibrateDialog
	st.InputSnapshot = st.Value
	st.Phase = dialog.DebounceCalibrateMeasuring
	st.MeasureStep = dialog.MeasureAwaitPress
	st.Samples = nil
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
	st.Focus = dialog.NewDialogTrailingButtonsForm(1, 3).MiddleButtonIndex()
	st.Status = ""
	st.MeasureStep = dialog.MeasureAwaitPress
	st.Samples = nil
	st.PressKey = ""
	st.EventCount = 0
}

func (a *App) finishDebounceCalibrateMeasuring() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	avg := dialog.AverageRepeatIntervalMS(st.Samples)
	ms := dialog.RecommendedDebounceMS(avg, dialog.CalibrationMarginMS())
	st.Phase = dialog.DebounceCalibrateEdit
	st.Value = dialog.FormatDebounceMS(ms)
	st.Cursor = utf8.RuneCountInString(st.Value)
	st.Focus = 0
	st.Status = fmt.Sprintf("Repeat interval %d ms; margin %d ms.", avg, dialog.CalibrationMarginMS())
	st.MeasureStep = dialog.MeasureAwaitPress
	st.Samples = nil
	st.PressKey = ""
	st.EventCount = 0
}

func (a *App) failDebounceCalibrateMeasuringTooSoon() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	st.MeasureStep = dialog.MeasureAwaitPress
	st.Samples = nil
	st.PressKey = ""
	st.EventCount = 0
	st.Status = fmt.Sprintf("Released too soon; hold until %d/%d on the bar.", 0, dialog.MeasureMinRepeatSamples())
}

func (a *App) clearDebounceCalibrateReleaseTimer() {
	a.debounceCalibrateRelease.Stop()
}

func (a *App) armDebounceCalibrateReleaseTimer() {
	a.debounceCalibrateRelease.Arm(dialog.MeasureReleaseIdle(), func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(debounceCalibrateReleasePayload{}))
	})
}

type debounceCalibrateReleasePayload struct{}

func (a *App) applyDebounceCalibrateReleasePayload() bool {
	st := &a.model.DebounceCalibrateDialog
	if !st.Open || st.Phase != dialog.DebounceCalibrateMeasuring || st.MeasureStep != dialog.MeasureCollecting {
		return false
	}
	if dialog.RepeatCalibrationReleaseReady(st.Samples) {
		a.finishDebounceCalibrateMeasuring()
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

	form := dialog.NewDialogTrailingButtonsForm(1, 3)
	if dialog.AltDialogOK(event) {
		a.applyDebounceCalibrateDialog()
		return
	}
	if altDialogCalibrate(event) {
		a.beginDebounceCalibrateMeasuring()
		return
	}
	if dialog.AltDialogCancel(event) {
		a.closeDebounceCalibrateDialog()
		return
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeDebounceCalibrateDialog()
		return
	case tcell.KeyEnter:
		switch st.Focus {
		case form.CancelIndex():
			a.closeDebounceCalibrateDialog()
		case form.MiddleButtonIndex():
			a.beginDebounceCalibrateMeasuring()
		default:
			a.applyDebounceCalibrateDialog()
		}
		return
	}

	if st.Focus == 0 {
		if a.handleDebounceCalibrateInputKey(event) {
			return
		}
	}

	if nf, ok := form.MoveFocus(st.Focus, event.Key()); ok {
		st.Focus = nf
	}
}

func altDialogCalibrate(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune && keymap.AltLetterModifiers(ev.Modifiers()) &&
		(ev.Rune() == 'l' || ev.Rune() == 'L')
}

func (a *App) handleDebounceCalibrateInputKey(event *tcell.EventKey) bool {
	st := &a.model.DebounceCalibrateDialog
	st.Status = ""
	switch event.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if st.Cursor > 0 {
			runes := []rune(st.Value)
			st.Value = string(runes[:st.Cursor-1]) + string(runes[st.Cursor:])
			st.Cursor--
		}
		return true
	case tcell.KeyDelete:
		runes := []rune(st.Value)
		if st.Cursor < len(runes) {
			st.Value = string(runes[:st.Cursor]) + string(runes[st.Cursor+1:])
		}
		return true
	case tcell.KeyLeft:
		if st.Cursor > 0 {
			st.Cursor--
		}
		return true
	case tcell.KeyRight:
		if st.Cursor < utf8.RuneCountInString(st.Value) {
			st.Cursor++
		}
		return true
	case tcell.KeyHome:
		st.Cursor = 0
		return true
	case tcell.KeyEnd:
		st.Cursor = utf8.RuneCountInString(st.Value)
		return true
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			return false
		}
		if !unicode.IsDigit(event.Rune()) {
			return true
		}
		runes := []rune(st.Value)
		runes = append(runes[:st.Cursor], append([]rune{event.Rune()}, runes[st.Cursor:]...)...)
		st.Value = string(runes)
		st.Cursor++
		return true
	default:
		return false
	}
}

func (a *App) handleDebounceCalibrateMeasuringKey(event *tcell.EventKey) {
	st := &a.model.DebounceCalibrateDialog
	switch event.Key() {
	case tcell.KeyEsc:
		a.abortDebounceCalibrateMeasuring()
		return
	}
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
		st.Samples = nil
		st.MeasureStep = dialog.MeasureCollecting
		st.Status = ""
		a.armDebounceCalibrateReleaseTimer()
	case dialog.MeasureCollecting:
		if fp != st.PressKey {
			return
		}
		hold := dialog.RecordRepeatCalibrationEvent(dialog.RepeatCalibrationHold{
			PressKey:    st.PressKey,
			LastEventAt: st.LastEventAt,
			EventCount:  st.EventCount,
			Samples:     st.Samples,
		}, fp, now)
		st.PressKey = hold.PressKey
		st.LastEventAt = hold.LastEventAt
		st.EventCount = hold.EventCount
		st.Samples = hold.Samples
		a.clearDebounceCalibrateReleaseTimer()
		a.armDebounceCalibrateReleaseTimer()
	}
}
