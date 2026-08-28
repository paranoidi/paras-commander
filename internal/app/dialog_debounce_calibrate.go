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
	value := dialog.FormatDebounceMS(a.config.UI.KeyRepeatDebounceMS)
	imageValue := dialog.FormatDebounceMS(a.config.UI.ImagePreviewDebounceMS)
	a.model.DebounceCalibrateDialog = dialog.DebounceCalibrateDialogState{
		Open:        true,
		Phase:       dialog.DebounceCalibrateEdit,
		Focus:       0,
		Value:       value,
		Cursor:      utf8.RuneCountInString(value),
		ImageValue:  imageValue,
		ImageCursor: utf8.RuneCountInString(imageValue),
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
		st.Focus = 0
		st.Status = fmt.Sprintf("Enter 0–%d", config.KeyRepeatDebounceMaxMS)
		return
	}
	imageMS, err := dialog.ParseDebounceMSInput(st.ImageValue)
	if err != nil {
		st.Focus = 1
		// The status row sits under the first field, so name the field this one is about.
		st.Status = fmt.Sprintf("Image preview: enter 0–%d", config.KeyRepeatDebounceMaxMS)
		return
	}
	a.config.UI.KeyRepeatDebounceMS = ms
	a.config.UI.ImagePreviewDebounceMS = imageMS
	a.closeDebounceCalibrateDialog()
	msg := fmt.Sprintf("Debounce set to %d ms (images %d ms)", ms, imageMS)
	patch := map[string]interface{}{
		"ui": map[string]interface{}{
			"key_repeat_debounce_ms":    ms,
			"image_preview_debounce_ms": imageMS,
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
	st.Focus = dialog.NewDialogTrailingButtonsForm(2, 3).MiddleButtonIndex()
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

	form := dialog.NewDialogTrailingButtonsForm(2, 3)
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

	switch st.Focus {
	case 0:
		if a.handleDebounceCalibrateInputKey(event, &st.Value, &st.Cursor) {
			return
		}
	case 1:
		if a.handleDebounceCalibrateInputKey(event, &st.ImageValue, &st.ImageCursor) {
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

// handleDebounceCalibrateInputKey edits one numeric field of the dialog; shared by both inputs.
func (a *App) handleDebounceCalibrateInputKey(event *tcell.EventKey, value *string, cursor *int) bool {
	a.model.DebounceCalibrateDialog.Status = ""
	switch event.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if *cursor > 0 {
			runes := []rune(*value)
			*value = string(runes[:*cursor-1]) + string(runes[*cursor:])
			*cursor--
		}
		return true
	case tcell.KeyDelete:
		runes := []rune(*value)
		if *cursor < len(runes) {
			*value = string(runes[:*cursor]) + string(runes[*cursor+1:])
		}
		return true
	case tcell.KeyLeft:
		if *cursor > 0 {
			*cursor--
		}
		return true
	case tcell.KeyRight:
		if *cursor < utf8.RuneCountInString(*value) {
			*cursor++
		}
		return true
	case tcell.KeyHome:
		*cursor = 0
		return true
	case tcell.KeyEnd:
		*cursor = utf8.RuneCountInString(*value)
		return true
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			return false
		}
		if !unicode.IsDigit(event.Rune()) {
			return true
		}
		runes := []rune(*value)
		runes = append(runes[:*cursor], append([]rune{event.Rune()}, runes[*cursor:]...)...)
		*value = string(runes)
		*cursor++
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
