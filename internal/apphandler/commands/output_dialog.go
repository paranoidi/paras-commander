package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// CloseOutputDialog closes the command-output dialog.
func (h *Handler) CloseOutputDialog() {
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{}
}

// HandleOutputDialogKey handles key events while the command-output dialog is open.
func (h *Handler) HandleOutputDialogKey(event *tcell.EventKey) {
	st := &h.model.CommandOutputDialog
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

// RunUserMenuCommandDialog runs argv and, on completion, opens the command-output dialog with
// its captured stdout/stderr. Intended to run in its own goroutine (spawned by a user-menu
// entry with dialog=true).
func (h *Handler) RunUserMenuCommandDialog(ctx context.Context, argv []string, workDir, title, prefWidth, prefHeight string) {
	defer h.EndBatch()

	select {
	case <-ctx.Done():
		return
	default:
	}

	res := cmdrun.Run(ctx, argv, workDir, cmdrun.MaxStreamBytes)

	if res.LaunchErr != nil {
		detail := res.LaunchErr.Error()
		log := "User menu: " + strings.TrimSpace(title) + ": " + detail
		h.PostWake(WakePayload{
			NotifyLog:            log,
			NotifyBanner:         log,
			NotifyUrg:            ui.MessageUrgencyError,
			ClearActiveSelection: true,
		})
		return
	}

	dialogTitle := strings.TrimSpace(title)

	stdout := strings.TrimRight(string(res.Stdout), "\n")
	var lines []string
	if stdout != "" {
		lines = strings.Split(stdout, "\n")
	}

	if res.ExitCode != 0 {
		if dialogTitle != "" {
			dialogTitle += fmt.Sprintf(" (exit %d)", res.ExitCode)
		} else {
			dialogTitle = fmt.Sprintf("exit %d", res.ExitCode)
		}
		stderr := strings.TrimSpace(string(res.Stderr))
		if stderr != "" {
			lines = append(lines, "--- stderr ---")
			lines = append(lines, strings.Split(stderr, "\n")...)
		}
	}

	st := dialog.CommandOutputDialogState{
		Open:       true,
		Title:      dialogTitle,
		Lines:      lines,
		PrefWidth:  prefWidth,
		PrefHeight: prefHeight,
	}
	h.PostWake(WakePayload{
		OpenOutputDialog:     &st,
		ClearActiveSelection: true,
	})
}
