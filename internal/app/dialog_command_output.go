package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) closeCommandOutputDialog() {
	a.model.CommandOutputDialog = dialog.CommandOutputDialogState{}
}

func (a *App) handleCommandOutputDialogKey(event *tcell.EventKey) {
	st := &a.model.CommandOutputDialog
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	visH := max(1, dialog.CommandOutputDialogListH(layout, *st))
	total := len(st.Lines)

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyEnter:
		a.closeCommandOutputDialog()
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
			a.closeCommandOutputDialog()
		}
	}
}

func (a *App) runUserMenuCommandDialog(ctx context.Context, argv []string, workDir, title, prefWidth, prefHeight string) {
	defer a.commandsBatchesInflight.Add(-1)

	select {
	case <-ctx.Done():
		return
	default:
	}

	res := cmdrun.Run(ctx, argv, workDir, cmdrun.MaxStreamBytes)

	if res.LaunchErr != nil {
		detail := res.LaunchErr.Error()
		log := "User menu: " + strings.TrimSpace(title) + ": " + detail
		a.postCommandWakePayload(commandWakePayload{
			notifyLog:            log,
			notifyBanner:         log,
			notifyUrg:            ui.MessageUrgencyError,
			clearActiveSelection: true,
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
	a.postCommandWakePayload(commandWakePayload{
		openOutputDialog:     &st,
		clearActiveSelection: true,
	})
}
