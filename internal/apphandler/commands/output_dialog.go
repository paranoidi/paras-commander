package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// CloseOutputDialog closes the command-output dialog.
func (h *Handler) CloseOutputDialog() {
	if cancel := h.model.CommandOutputDialog.Cancel; cancel != nil {
		cancel()
	}
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{}
}

// OpenRunningOutputDialog opens the output dialog in its running state for the Commands row
// rowID. cancel kills the run; queued shows it as waiting for a pool slot until
// WakePayload.OutputDialogStartedRunID arrives. Main goroutine only.
func (h *Handler) OpenRunningOutputDialog(rowID, title string, queued bool, cancel context.CancelFunc, prefW, prefH string) {
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{
		Open:       true,
		Running:    true,
		Queued:     queued,
		RunID:      rowID,
		Cancel:     cancel,
		Title:      strings.TrimSpace(title),
		PrefWidth:  prefW,
		PrefHeight: prefH,
	}
}

// closeOutputDialogKeepRunning closes the dialog without canceling the run (Background).
func (h *Handler) closeOutputDialogKeepRunning() {
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{}
}

// HandleOutputDialogKey handles key events while the command-output dialog is open.
func (h *Handler) HandleOutputDialogKey(event *tcell.EventKey) {
	st := &h.model.CommandOutputDialog
	if st.Running {
		switch {
		case event.Key() == tcell.KeyEsc || dialog.AltLetter(event, 'b'):
			h.closeOutputDialogKeepRunning()
		case dialog.AltDialogCancel(event):
			h.CloseOutputDialog()
		case event.Key() == tcell.KeyLeft:
			st.Focus = 0
		case event.Key() == tcell.KeyRight:
			st.Focus = 1
		case event.Key() == tcell.KeyEnter:
			if st.Focus == 0 {
				h.closeOutputDialogKeepRunning()
			} else {
				h.CloseOutputDialog()
			}
		}
		return
	}

	w, ht := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(w, ht)
	visH := max(1, dialog.CommandOutputDialogListH(layout, *st))
	total := len(st.Lines)

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyEnter:
		h.CloseOutputDialog()
	case tcell.KeyUp:
		if st.Scroll > 0 {
			st.Scroll--
		}
	case tcell.KeyDown:
		if st.Scroll < total-visH {
			st.Scroll++
		}
	case tcell.KeyPgUp:
		st.Scroll = max(0, st.Scroll-visH)
	case tcell.KeyPgDn:
		st.Scroll = max(0, min(max(total-visH, 0), st.Scroll+visH))
	case tcell.KeyRune:
		if dialog.AltDialogOK(event) || dialog.AltDialogCancel(event) {
			h.CloseOutputDialog()
		}
	}
}

// OutputDialogResult builds the finished-state output dialog for a completed run.
func OutputDialogResult(runID, title string, res cmdrun.RunResult, prefW, prefH string) dialog.CommandOutputDialogState {
	dialogTitle := strings.TrimSpace(title)
	stdout := strings.TrimRight(string(res.Stdout), "\n")
	var lines []string
	if stdout != "" {
		lines = strings.Split(stdout, "\n")
	}
	stderr := strings.TrimSpace(string(res.Stderr))
	if res.LaunchErr != nil {
		stderr = res.LaunchErr.Error()
	} else if res.ExitCode != 0 {
		if dialogTitle != "" {
			dialogTitle += fmt.Sprintf(" (exit %d)", res.ExitCode)
		} else {
			dialogTitle = fmt.Sprintf("exit %d", res.ExitCode)
		}
	}
	if stderr != "" {
		lines = append(lines, "--- stderr ---")
		lines = append(lines, strings.Split(stderr, "\n")...)
	}
	return dialog.CommandOutputDialogState{
		Open:       true,
		RunID:      runID,
		Title:      dialogTitle,
		Lines:      lines,
		PrefWidth:  prefW,
		PrefHeight: prefH,
	}
}
