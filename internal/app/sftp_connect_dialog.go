package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) loadSSHConfig() sshconfig.Config {
	path, err := sshconfig.ResolvePath(a.config.SFTP.SSHConfigFile)
	if err != nil {
		return sshconfig.Config{}
	}
	cfg, err := sshconfig.Load(path)
	if err != nil {
		return sshconfig.Config{Path: path}
	}
	return cfg
}

func (a *App) openSFTPConnectDialogForPanel(panelID int) {
	a.sftpConnectTargetPanel = panelID
	sshCfg := a.loadSSHConfig()
	a.sftpConnectHosts = append([]sshconfig.HostEntry(nil), sshCfg.Entries...)

	display := sshconfig.FormatHostListLines(a.sftpConnectHosts)

	prefill := "sftp://"
	a.model.SFTPConnectDialog = ui.SFTPConnectDialogState{
		Open:         true,
		PanelID:      panelID,
		DisplayLines: display,
		Selected:     0,
		Location: ui.FileDialogField{
			Label:          "Location",
			Value:          prefill,
			Prefill:        prefill,
			Cursor:         len([]rune(prefill)),
			PrefillPending: true,
		},
		Focus: 0,
	}
	a.syncSFTPConnectDialogRanks()
	if len(a.model.SFTPConnectDialog.Ranked) == 0 {
		a.model.SFTPConnectDialog.Focus = 1
	}
	ui.EnsureSFTPConnectListScroll(&a.model.SFTPConnectDialog, a.sftpConnectListRows())
	a.model.FileDialog = ui.FileDialogState{}
	a.clearTransientMessage()
}

func (a *App) closeSFTPConnectDialog() {
	a.model.SFTPConnectDialog = ui.SFTPConnectDialogState{}
	a.sftpConnectHosts = nil
}

func (a *App) syncSFTPConnectDialogRanks() {
	st := &a.model.SFTPConnectDialog
	if !st.Open {
		return
	}
	lines := make([]string, len(st.DisplayLines))
	copy(lines, st.DisplayLines)
	q := search.Parse(st.Query)
	opts := search.Options{CaseInsensitive: a.config.CaseInsensitiveFilter}
	ranked := q.Rank(lines, opts)
	st.Ranked = make([]int, len(ranked))
	st.MatchRanges = make([][]search.Range, len(st.DisplayLines))
	for i := range st.MatchRanges {
		st.MatchRanges[i] = nil
	}
	for i, r := range ranked {
		st.Ranked[i] = r.Index
		if r.Index >= 0 && r.Index < len(st.MatchRanges) {
			st.MatchRanges[r.Index] = r.Result.Ranges
		}
	}
	if st.Selected >= len(st.Ranked) {
		if len(st.Ranked) == 0 {
			st.Selected = 0
		} else {
			st.Selected = len(st.Ranked) - 1
		}
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
}

func (a *App) sftpConnectListRows() int {
	_, h := a.screen.Size()
	listH := h - 16
	if listH > 12 {
		return 12
	}
	if listH < 3 {
		return 3
	}
	return listH
}

func (a *App) applySFTPConnectHostToLocation() {
	st := &a.model.SFTPConnectDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	idx := st.Ranked[st.Selected]
	if idx < 0 || idx >= len(a.sftpConnectHosts) {
		return
	}
	uri, err := a.sftpConnectHosts[idx].BuildSFTPURI("")
	if err != nil {
		a.setErrorMessage("SFTP link", err)
		return
	}
	st.Location.Value = uri
	st.Location.Prefill = uri
	st.Location.Cursor = len([]rune(uri))
	st.Location.PrefillPending = false
}

func (a *App) executeSFTPConnectDialog() {
	st := &a.model.SFTPConnectDialog
	if len(st.Ranked) > 0 && strings.TrimSpace(st.Location.Value) == strings.TrimSpace(st.Location.Prefill) {
		a.applySFTPConnectHostToLocation()
	}
	raw := strings.TrimSpace(st.Location.Value)
	panelID := st.PanelID
	a.closeSFTPConnectDialog()
	if raw == "" {
		a.setErrorMessage("SFTP link", fmt.Errorf("empty location"))
		return
	}
	a.executeSFTPConnectURI(panelID, raw)
}

func (a *App) handleSFTPConnectDialogKey(event *tcell.EventKey) {
	if ui.AltDialogOK(event) {
		a.executeSFTPConnectDialog()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeSFTPConnectDialog()
		return
	}

	st := &a.model.SFTPConnectDialog
	if st.Focus == 0 {
		onChange := func() {
			a.syncSFTPConnectDialogRanks()
			st.Selected = 0
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}
		if a.handleScrollingQueryKey(event, true, sftpConnectDialogScrollingQuery(st, a.sftpConnectDialogQueryWidth(), onChange)) {
			return
		}
	}
	if st.Focus == 1 && a.editTransferFieldKey(event, &st.Location) {
		return
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeSFTPConnectDialog()
	case tcell.KeyEnter:
		switch st.Focus {
		case 3:
			a.closeSFTPConnectDialog()
		case 2:
			a.executeSFTPConnectDialog()
		case 1:
			a.executeSFTPConnectDialog()
		default:
			a.applySFTPConnectHostToLocation()
			if st.Focus == 0 {
				st.Focus = 1
			}
		}
	case tcell.KeyTab:
		st.Focus = (st.Focus + 1) % 4
		if st.Focus == 0 && len(st.Ranked) == 0 {
			st.Focus = 1
		}
	case tcell.KeyBacktab:
		st.Focus--
		if st.Focus < 0 {
			st.Focus = 3
		}
		if st.Focus == 0 && len(st.Ranked) == 0 {
			st.Focus = 3
		}
	case tcell.KeyLeft:
		if st.Focus >= 2 {
			if st.Focus > 2 {
				st.Focus--
			}
		}
	case tcell.KeyRight:
		if st.Focus >= 2 && st.Focus < 3 {
			st.Focus++
		}
	case tcell.KeyUp:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		} else {
			a.sftpConnectDialogMoveFocus(-1)
		}
	case tcell.KeyDown:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		} else {
			a.sftpConnectDialogMoveFocus(1)
		}
	case tcell.KeyHome:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = 0
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}
	case tcell.KeyEnd:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}
	case tcell.KeyPgUp:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.sftpConnectListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -step)
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}
	case tcell.KeyPgDn:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.sftpConnectListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), step)
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}
	}
}

func (a *App) sftpConnectDialogMoveFocus(delta int) {
	st := &a.model.SFTPConnectDialog
	if delta < 0 {
		switch st.Focus {
		case 0:
			return
		case 1:
			if len(st.Ranked) > 0 {
				st.Focus = 0
			}
		case 2:
			st.Focus = 1
		case 3:
			st.Focus = 2
		}
		return
	}
	switch st.Focus {
	case 0:
		st.Focus = 1
	case 1:
		st.Focus = 2
	case 2, 3:
		return
	}
}
