package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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
	a.sftp.connectTargetPanel = panelID
	sshCfg := a.loadSSHConfig()
	a.sftp.connectHosts = append([]sshconfig.HostEntry(nil), sshCfg.Entries...)

	display := sshconfig.FormatHostListLines(a.sftp.connectHosts)

	prefill := "sftp://"
	a.model.SFTPConnectDialog = dialog.SFTPConnectDialogState{
		Open:         true,
		PanelID:      panelID,
		DisplayLines: display,
		Selected:     0,
		Location: dialog.FileDialogField{
			Label:          "Location",
			Value:          prefill,
			Prefill:        prefill,
			Cursor:         len([]rune(prefill)),
			PrefillPending: true,
		},
		Focus: dialog.SFTPFilter,
	}
	a.syncSFTPConnectDialogRanks()
	if len(a.model.SFTPConnectDialog.Ranked) == 0 {
		a.model.SFTPConnectDialog.Focus = dialog.SFTPLocation
	}
	dialog.EnsureSFTPConnectListScroll(&a.model.SFTPConnectDialog, a.sftpConnectListRows())
	a.model.FileDialog = dialog.FileDialogState{}
	a.clearTransientMessage()
}

func (a *App) closeSFTPConnectDialog() {
	a.model.SFTPConnectDialog = dialog.SFTPConnectDialogState{}
	a.sftp.connectHosts = nil
}

func (a *App) syncSFTPConnectDialogRanks() {
	st := &a.model.SFTPConnectDialog
	if !st.Open {
		return
	}
	lines := make([]string, len(st.DisplayLines))
	copy(lines, st.DisplayLines)
	st.Ranked, st.MatchRanges = syncFilteredListRanks(lines, st.Query, len(st.DisplayLines), a.config.Filter.CaseInsensitive)
	clampFilteredListSelection(&st.Selected, len(st.Ranked))
	dialog.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
}

func (a *App) sftpConnectListRows() int {
	w, h := a.screen.Size()
	return max(dialog.SFTPConnectListRows(a.layoutForTerminalSize(w, h)), 1)
}

func (a *App) sftpConnectView() dlg.Spec {
	w, h := a.screen.Size()
	return dialog.SFTPConnectView(a.model.SFTPConnectDialog, a.layoutForTerminalSize(w, h), a.styles)
}

func (a *App) applySFTPConnectHostToLocation() {
	st := &a.model.SFTPConnectDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	idx := st.Ranked[st.Selected]
	if idx < 0 || idx >= len(a.sftp.connectHosts) {
		return
	}
	uri, err := a.sftp.connectHosts[idx].BuildSFTPURI("")
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
	st := &a.model.SFTPConnectDialog
	if ev, ok := dlg.HandleKey(a.sftpConnectView(), &st.Focus, event); ok {
		a.applySFTPConnectEvent(ev)
		return
	}
	a.editSFTPConnectDialog(event)
}

// applySFTPConnectEvent acts on a widget event of the SFTP dialog (key or click).
func (a *App) applySFTPConnectEvent(ev dlg.Event) {
	st := &a.model.SFTPConnectDialog
	switch {
	case ev.Kind == dlg.EventCaret && ev.ID == dialog.SFTPFilter:
		st.QueryCursor = ev.Index
	case ev.Kind == dlg.EventCaret:
		st.Location.SetCursor(ev.Index)
	case ev.Kind == dlg.EventListMove:
		st.Selected, st.ListScroll = ev.Index, ev.Offset
	case ev.ID == dlg.Cancel:
		a.closeSFTPConnectDialog()
	case ev.ID == dlg.OK || ev.ID == dialog.SFTPLocation:
		a.executeSFTPConnectDialog()
	case ev.ID == dialog.SFTPFilter: // Enter in the filter takes the selected host to the location
		a.applySFTPConnectHostToLocation()
		st.Focus = dialog.SFTPLocation
	}
}

// editSFTPConnectDialog feeds a key the engine declined to the focused input's text editing.
func (a *App) editSFTPConnectDialog(event *tcell.EventKey) {
	st := &a.model.SFTPConnectDialog
	switch st.Focus {
	case dialog.SFTPFilter:
		onChange := func() {
			a.syncSFTPConnectDialogRanks()
			st.Selected = 0
			dialog.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, a.sftpConnectDialogQueryWidth(), onChange)
		a.handleScrollingQueryKey(event, true, edit)
	case dialog.SFTPLocation:
		dialog.HandleFileDialogFieldKey(event, &st.Location, a.keys.DialogInput, nil)
	}
}
