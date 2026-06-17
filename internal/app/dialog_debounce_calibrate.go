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
)

func (a *App) openDebounceCalibrateDialog() {
	a.clearTransientMessage()
	ms := a.config.UI.KeyRepeatDebounceMS
	a.model.DebounceCalibrateDialog = ui.DebounceCalibrateDialogState{
		Open:   true,
		Phase:  ui.DebounceCalibrateEdit,
		Focus:  0,
		Value:  ui.FormatDebounceMS(ms),
		Cursor: utf8.RuneCountInString(ui.FormatDebounceMS(ms)),
	}
}

func (a *App) closeDebounceCalibrateDialog() {
	a.clearDebounceCalibrateReleaseTimer()
	a.model.DebounceCalibrateDialog = ui.DebounceCalibrateDialogState{}
}

func (a *App) applyDebounceCalibrateDialog() {
	st := &a.model.DebounceCalibrateDialog
	ms, err := ui.ParseDebounceMSInput(st.Value)
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
	st.Phase = ui.DebounceCalibrateMeasuring
	st.MeasureStep = ui.MeasureAwaitPress
	st.Samples = nil
	st.PressKey = ""
	st.EventCount = 0
	st.Status = ""
	a.clearDebounceCalibrateReleaseTimer()
}

func (a *App) abortDebounceCalibrateMeasuring() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	st.Phase = ui.DebounceCalibrateEdit
	st.Value = st.InputSnapshot
	st.Cursor = utf8.RuneCountInString(st.Value)
	st.Focus = ui.NewDialogTrailingButtonsForm(1, 3).MiddleButtonIndex()
	st.Status = ""
	st.MeasureStep = ui.MeasureAwaitPress
	st.Samples = nil
	st.PressKey = ""
	st.EventCount = 0
}

func (a *App) finishDebounceCalibrateMeasuring() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	avg := ui.AverageRepeatIntervalMS(st.Samples)
	ms := ui.RecommendedDebounceMS(avg, ui.CalibrationMarginMS())
	st.Phase = ui.DebounceCalibrateEdit
	st.Value = ui.FormatDebounceMS(ms)
	st.Cursor = utf8.RuneCountInString(st.Value)
	st.Focus = 0
	st.Status = fmt.Sprintf("Repeat interval %d ms; margin %d ms.", avg, ui.CalibrationMarginMS())
	st.MeasureStep = ui.MeasureAwaitPress
	st.Samples = nil
	st.PressKey = ""
	st.EventCount = 0
}

func (a *App) failDebounceCalibrateMeasuringTooSoon() {
	st := &a.model.DebounceCalibrateDialog
	a.clearDebounceCalibrateReleaseTimer()
	st.MeasureStep = ui.MeasureAwaitPress
	st.Samples = nil
	st.PressKey = ""
	st.EventCount = 0
	st.Status = fmt.Sprintf("Released too soon; hold until %d/%d on the bar.", 0, ui.MeasureMinRepeatSamples())
}

func (a *App) clearDebounceCalibrateReleaseTimer() {
	a.debounceCalibrateReleaseMu.Lock()
	if a.debounceCalibrateReleaseTimer != nil {
		if !a.debounceCalibrateReleaseTimer.Stop() {
			select {
			case <-a.debounceCalibrateReleaseTimer.C:
			default:
			}
		}
		a.debounceCalibrateReleaseTimer = nil
	}
	a.debounceCalibrateReleaseMu.Unlock()
}

func (a *App) armDebounceCalibrateReleaseTimer() {
	delay := ui.MeasureReleaseIdle()
	a.debounceCalibrateReleaseMu.Lock()
	if a.debounceCalibrateReleaseTimer != nil {
		if !a.debounceCalibrateReleaseTimer.Stop() {
			select {
			case <-a.debounceCalibrateReleaseTimer.C:
			default:
			}
		}
	}
	a.debounceCalibrateReleaseTimer = time.AfterFunc(delay, func() {
		a.debounceCalibrateReleaseMu.Lock()
		a.debounceCalibrateReleaseTimer = nil
		a.debounceCalibrateReleaseMu.Unlock()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(debounceCalibrateReleasePayload{}))
	})
	a.debounceCalibrateReleaseMu.Unlock()
}

type debounceCalibrateReleasePayload struct{}

func (a *App) applyDebounceCalibrateReleasePayload() bool {
	st := &a.model.DebounceCalibrateDialog
	if !st.Open || st.Phase != ui.DebounceCalibrateMeasuring || st.MeasureStep != ui.MeasureCollecting {
		return false
	}
	if ui.RepeatCalibrationReleaseReady(st.Samples) {
		a.finishDebounceCalibrateMeasuring()
		return true
	}
	a.failDebounceCalibrateMeasuringTooSoon()
	return true
}

func (a *App) handleDebounceCalibrateDialogKey(event *tcell.EventKey) {
	st := &a.model.DebounceCalibrateDialog
	if st.Phase == ui.DebounceCalibrateMeasuring {
		a.handleDebounceCalibrateMeasuringKey(event)
		return
	}

	form := ui.NewDialogTrailingButtonsForm(1, 3)
	if ui.AltDialogOK(event) {
		a.applyDebounceCalibrateDialog()
		return
	}
	if altDialogCalibrate(event) {
		a.beginDebounceCalibrateMeasuring()
		return
	}
	if ui.AltDialogCancel(event) {
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
	fp, ok := ui.KeyFingerprint(event)
	if !ok {
		return
	}
	now := time.Now()
	switch st.MeasureStep {
	case ui.MeasureAwaitPress:
		st.PressKey = fp
		st.LastEventAt = now
		st.EventCount = 1
		st.Samples = nil
		st.MeasureStep = ui.MeasureCollecting
		st.Status = ""
		a.armDebounceCalibrateReleaseTimer()
	case ui.MeasureCollecting:
		if fp != st.PressKey {
			return
		}
		hold := ui.RecordRepeatCalibrationEvent(ui.RepeatCalibrationHold{
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
