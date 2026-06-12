package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func metaDialogEntries(mf *metacmds.MetaFile) []ui.MetaEntry {
	return metaEntries(mf)
}

func (a *App) resolveMetaEditPath(panelID int) (string, error) {
	metaPath, warns := metacmds.ResolveMetaTOML(a.config, a.model.UserHomeDir, a.metaConfigDir(), a.panelByID(panelID).PathString())
	for _, w := range warns {
		a.setTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if metaPath == "" {
		return a.ensureGlobalMetaStub()
	}
	return metaPath, nil
}

func (a *App) editMetaConfigFromDialog() {
	st := &a.model.MetaDialog
	if !st.Open {
		return
	}
	path, err := a.resolveMetaEditPath(st.PanelID)
	if err != nil {
		a.setErrorMessage("Meta commands", err)
		return
	}
	if a.openMetaFileEditor(path) {
		a.refreshMetaDialogAfterConfigEdit()
	}
}

func (a *App) refreshMetaDialogAfterConfigEdit() {
	st := &a.model.MetaDialog
	if !st.Open {
		return
	}
	panelID := st.PanelID
	prevChecked := make(map[string]bool, len(st.Entries))
	for i, e := range st.Entries {
		if i < len(st.Checked) && st.Checked[i] {
			prevChecked[e.Name] = true
		}
	}
	prevFocusName, prevFocusButton := metaDialogFocusTarget(st.Entries, st.Focus)

	mf := a.loadMetaFile(panelID)
	entries := metaDialogEntries(mf)
	checked := make([]bool, len(entries))
	for i, e := range entries {
		checked[i] = prevChecked[e.Name]
	}

	n := len(entries)
	form := ui.NewDialogLinearForm(n)
	focus := metaDialogFocusFromTarget(entries, form, prevFocusName, prevFocusButton, 0)

	st.Entries = entries
	st.Checked = checked
	st.Focus = focus
}

func metaEntryIndexByName(entries []ui.MetaEntry, name string) int {
	if name == "" {
		return -1
	}
	for i, e := range entries {
		if e.Name == name {
			return i
		}
	}
	return -1
}

func metaDialogFocusTarget(entries []ui.MetaEntry, focus int) (entryName string, button tcell.Key) {
	n := len(entries)
	form := ui.NewDialogLinearForm(n)
	switch {
	case focus < n:
		return entries[focus].Name, 0
	case focus == form.OKIndex():
		return "", tcell.KeyEnter
	case focus == form.CancelIndex():
		return "", tcell.KeyEsc
	default:
		return "", 0
	}
}

func metaDialogFocusFromTarget(entries []ui.MetaEntry, form ui.DialogLinearForm, entryName string, button tcell.Key, _ int) int {
	switch button {
	case tcell.KeyEnter:
		return form.OKIndex()
	case tcell.KeyEsc:
		return form.CancelIndex()
	}
	if entryName != "" {
		if i := metaEntryIndexByName(entries, entryName); i >= 0 {
			return i
		}
	}
	return 0
}

func (a *App) resolveUserMenuEditPath() (string, error) {
	menuPath, warns := a.resolveUserMenuContext()
	for _, w := range warns {
		a.setTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if menuPath == "" {
		return a.ensureGlobalUserMenuStub()
	}
	return menuPath, nil
}

func (a *App) editUserMenuConfigFromDialog() {
	st := &a.model.UserMenu
	if !st.Open {
		return
	}
	path := st.SourcePath
	if path == "" {
		var err error
		path, err = a.resolveUserMenuEditPath()
		if err != nil {
			a.setErrorMessage("User menu", err)
			return
		}
	}
	if a.openUserMenuEditor(path) {
		a.reloadUserMenuDialog()
	}
}

func (a *App) reloadUserMenuDialog() {
	st := &a.model.UserMenu
	if !st.Open {
		return
	}
	prevTitle := ""
	prevKey := ""
	if st.Selected >= 0 && st.Selected < len(st.Entries) {
		prevTitle = st.Entries[st.Selected].Title
		prevKey = st.Entries[st.Selected].Key
	}
	oldForm := dialog.NewUserMenuDialogForm(len(st.Entries))
	wasOnCancel := st.Focus == oldForm.CancelIndex()

	menuPath := st.SourcePath
	if menuPath == "" {
		var err error
		menuPath, err = a.resolveUserMenuEditPath()
		if err != nil {
			a.setErrorMessage("User menu", err)
			a.closeUserMenu()
			return
		}
	}

	mf, err := usermenu.LoadFile(menuPath)
	if err != nil {
		a.setUserMenuCritical(err)
		a.closeUserMenu()
		return
	}
	if len(mf.Entries) == 0 {
		a.setTransientMessage("User menu: no entries (edit with Shift+F2)", ui.MessageUrgencyWarn)
		a.closeUserMenu()
		return
	}

	active := a.panelByID(a.model.ActivePanel)
	other := a.panelByID(a.inactivePanelID())
	ctx := &usermenu.EvalContext{Active: active, Other: other}
	visible, defIdx, err := usermenu.FilterVisible(mf, ctx)
	if err != nil {
		a.setUserMenuCritical(err)
		a.closeUserMenu()
		return
	}
	if len(visible) == 0 {
		a.setTransientMessage("User menu: no visible entries", ui.MessageUrgencyWarn)
		a.closeUserMenu()
		return
	}

	selected := userMenuEntryIndexByKeyOrTitle(visible, prevKey, prevTitle, defIdx)
	form := dialog.NewUserMenuDialogForm(len(visible))
	focus := selected
	if wasOnCancel {
		focus = form.CancelIndex()
	}

	st.Entries = visible
	st.Selected = selected
	st.Focus = focus
	st.SourcePath = menuPath

	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	vr := dialog.UserMenuListViewportRows(layout, len(visible))
	dialog.UserMenuEnsureScroll(st, vr)
}

func userMenuEntryIndexByKeyOrTitle(entries []usermenu.MenuEntry, key, title string, fallback int) int {
	if key != "" {
		for i, e := range entries {
			if e.Key == key {
				return i
			}
		}
	}
	if title != "" {
		for i, e := range entries {
			if e.Title == title {
				return i
			}
		}
	}
	if fallback >= 0 && fallback < len(entries) {
		return fallback
	}
	return 0
}
