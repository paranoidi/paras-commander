package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
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
	st.Ranked, st.MatchRanges = syncFilteredListRanks(lines, st.Query, len(st.DisplayLines), a.config.CaseInsensitiveFilter)
	clampFilteredListSelection(&st.Selected, len(st.Ranked))
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
		a.setErrorMessage("SFTP", err)
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
		a.setErrorMessage("SFTP", fmt.Errorf("empty location"))
		return
	}
	a.executeSFTPConnectURI(panelID, raw)
}

func (a *App) handleSFTPConnectDialogKey(event *tcell.EventKey) {
	if a.tryStandardDialogActions(event, a.executeSFTPConnectDialog, a.closeSFTPConnectDialog, nil) {
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
	if st.Focus == 1 && a.handleFileDialogFieldKey(event, &st.Location, nil) {
		return
	}

	form := ui.NewDialogLinearForm(2)
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
	case tcell.KeyTab, tcell.KeyBacktab:
		if nf, ok := form.MoveFocus(st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && len(st.Ranked) == 0 {
				st.Focus = 1
			}
		}
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if st.Focus >= form.OKIndex() {
			if nf, ok := form.MoveFocus(st.Focus, event.Key()); ok {
				st.Focus = nf
			}
			break
		}
		if st.Focus == 0 {
			if event.Key() == tcell.KeyDown && len(st.Ranked) == 0 {
				st.Focus = 1
				break
			}
			if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.sftpConnectListRows, func() {
				ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
			}) {
				break
			}
			if event.Key() == tcell.KeyUp {
				break
			}
		}
		if nf, ok := form.MoveFocus(st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && len(st.Ranked) == 0 {
				if event.Key() == tcell.KeyUp {
					st.Focus = form.CancelIndex()
				} else {
					st.Focus = 1
				}
			}
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.sftpConnectListRows, func() {
			ui.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}) {
			break
		}
	}
}
