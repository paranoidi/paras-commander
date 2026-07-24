package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
		Focus: 0,
	}
	a.syncSFTPConnectDialogRanks()
	if len(a.model.SFTPConnectDialog.Ranked) == 0 {
		a.model.SFTPConnectDialog.Focus = 1
	}
	dialog.EnsureSFTPConnectListScroll(&a.model.SFTPConnectDialog, a.sftpConnectListRows())
	a.model.FileDialog = dialog.FileDialogState{}
	a.clearTransientMessage()
}

func (a *App) closeSFTPConnectDialog() {
	a.model.SFTPConnectDialog = dialog.SFTPConnectDialogState{}
	a.sftpConnectHosts = nil
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

// sftpConnectNavFocus applies the generic Left/Right/Up/Down focus transition via form,
// redirecting a landing on the host list (0) back to the location field when the list is
// empty (or, moving up into it, to Cancel) — mirroring pathPickerNavFocus's hidden-focus
// skip for the path picker's Navigate purpose.
func sftpConnectNavFocus(form dialog.DialogLinearForm, focus int, key tcell.Key, emptyList bool) (int, bool) {
	nf, ok := form.MoveFocus(focus, key)
	if !ok {
		return focus, false
	}
	if nf == 0 && emptyList {
		if key == tcell.KeyUp {
			return form.CancelIndex(), true
		}
		return 1, true
	}
	return nf, true
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
			dialog.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, a.sftpConnectDialogQueryWidth(), onChange)
		if a.handleScrollingQueryKey(event, true, edit) {
			return
		}
	}
	if st.Focus == 1 && a.handleFileDialogFieldKey(event, &st.Location, nil) {
		return
	}

	form := dialog.NewDialogLinearForm(2)
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
		emptyList := len(st.Ranked) == 0
		if st.Focus >= form.OKIndex() {
			if nf, ok := sftpConnectNavFocus(form, st.Focus, event.Key(), emptyList); ok {
				st.Focus = nf
			}
			break
		}
		if st.Focus == 0 {
			if event.Key() == tcell.KeyDown && emptyList {
				st.Focus = 1
				break
			}
			if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.sftpConnectListRows, func() {
				dialog.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
			}) {
				break
			}
			if event.Key() == tcell.KeyUp {
				break
			}
		}
		if nf, ok := sftpConnectNavFocus(form, st.Focus, event.Key(), emptyList); ok {
			st.Focus = nf
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.sftpConnectListRows, func() {
			dialog.EnsureSFTPConnectListScroll(st, a.sftpConnectListRows())
		}) {
			break
		}
	}
}
