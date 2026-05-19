package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openMassRenameDialog(p *panel.State) {
	src, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Mass rename", err)
		return
	}
	if len(src.Entries) == 0 {
		a.setErrorMessage("Mass rename", fmt.Errorf("no files selected"))
		return
	}
	sources := make([]ui.MassRenameSource, 0, len(src.Entries))
	for _, e := range src.Entries {
		ap := e.Path
		if !filepath.IsAbs(ap) {
			ap = filepath.Join(p.Path, ap)
		}
		ap = filepath.Clean(ap)
		sources = append(sources, ui.MassRenameSource{Path: ap, Name: filepath.Base(ap)})
	}
	fields := []ui.FileDialogField{
		{Label: "Find", Value: "", Cursor: 0},
		{Label: "Replace", Value: "", Cursor: 0},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:                    true,
		DialogType:              ui.FileDialogMassRename,
		Fields:                  fields,
		FocusedField:            0,
		MassRenameMode:          ui.MassRenameModeUISimple,
		MassRenameCaseFold:      false,
		MassRenamePreviewScroll: 0,
		MassRenameSources:       sources,
		MassRenamePreviewBefore: nil,
		MassRenamePreviewAfter:  nil,
	}
	a.massRenameSyncFieldLabels()
	a.recomputeMassRenamePreview()
}

func (a *App) massRenameSyncFieldLabels() {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogMassRename || len(d.Fields) < 2 {
		return
	}
	if d.MassRenameMode == ui.MassRenameModeUISimple {
		d.Fields[0].Label = "Find"
		d.Fields[1].Label = "Replace"
	} else {
		d.Fields[0].Label = "Pattern"
		d.Fields[1].Label = "Replacement"
	}
}

func (a *App) recomputeMassRenamePreview() {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogMassRename {
		return
	}
	d.Message = ""
	d.MassRenamePatternCompileHint = ""
	if len(d.Fields) > 0 {
		d.Fields[0].InputInvalid = false
	}
	if len(d.MassRenameSources) == 0 {
		d.MassRenamePreviewBefore = nil
		d.MassRenamePreviewAfter = nil
		return
	}
	entries := make([]localfs.Entry, len(d.MassRenameSources))
	for i, s := range d.MassRenameSources {
		entries[i] = localfs.Entry{Name: s.Name, Path: s.Path, Type: localfs.EntryFile}
	}
	panelPath := a.activePanel().Path
	find, replace := "", ""
	if len(d.Fields) > 0 {
		find = d.Fields[0].Value
	}
	if len(d.Fields) > 1 {
		replace = d.Fields[1].Value
	}
	mode := ops.MassRenameModeSimple
	var rx *regexp.Regexp
	if d.MassRenameMode == ui.MassRenameModeUIRegex {
		mode = ops.MassRenameModeRegex
		if strings.TrimSpace(find) == "" {
			rx = nil
		} else {
			var err error
			rx, err = ops.MassRenameCompileRegex(find)
			if err != nil {
				if len(d.Fields) > 0 {
					d.Fields[0].InputInvalid = true
				}
				d.MassRenamePatternCompileHint = ops.MassRenameRegexCompileUserMessage(err)
				rx = nil // preview as no-op transform; error is shown on the pattern field only
			}
		}
	}
	caseFold := d.MassRenameCaseFold && d.MassRenameMode == ui.MassRenameModeUISimple
	rows, err := ops.MassRenameCompute(entries, panelPath, mode, find, replace, caseFold, rx)
	if err != nil {
		d.Message = err.Error()
		d.MassRenamePreviewBefore = []string{"! " + err.Error()}
		d.MassRenamePreviewAfter = []string{""}
		return
	}
	if err := ops.MassRenameValidateRows(rows); err != nil {
		d.Message = err.Error()
	}
	before := make([]string, 0, len(rows)+1)
	after := make([]string, 0, len(rows)+1)
	if d.Message != "" {
		before = append(before, "! "+d.Message)
		after = append(after, "")
	}
	for _, r := range rows {
		before = append(before, r.OldBase)
		after = append(after, r.NewBase)
	}
	d.MassRenamePreviewBefore = before
	d.MassRenamePreviewAfter = after
	_, h := a.screen.Size()
	vp := ui.MassRenamePreviewViewportRows(h)
	ui.MassRenameEnsurePreviewScroll(&a.model.FileDialog, vp, len(before))
}

func (a *App) applyMassRenameModeFromFocus() {
	d := &a.model.FileDialog
	switch d.FocusedField {
	case 0:
		d.MassRenameMode = ui.MassRenameModeUISimple
	case 1:
		d.MassRenameMode = ui.MassRenameModeUIRegex
	}
	if d.MassRenameMode == ui.MassRenameModeUIRegex && d.FocusedField == 4 {
		d.FocusedField = 3
	}
	a.massRenameSyncFieldLabels()
	a.recomputeMassRenamePreview()
}

func (a *App) executeMassRename() {
	d := &a.model.FileDialog
	a.recomputeMassRenamePreview()
	if d.DialogType == ui.FileDialogMassRename &&
		d.MassRenameMode == ui.MassRenameModeUIRegex &&
		len(d.Fields) > 0 && d.Fields[0].InputInvalid {
		find := d.Fields[0].Value
		if _, err := ops.MassRenameCompileRegex(find); err != nil {
			msg := ops.MassRenameRegexCompileUserMessage(err)
			if msg == "" {
				msg = err.Error()
			}
			a.setTransientMessage(msg, ui.MessageUrgencyWarn)
		} else {
			a.setTransientMessage("Invalid regular expression", ui.MessageUrgencyWarn)
		}
		return
	}
	if strings.TrimSpace(d.Message) != "" {
		a.setTransientMessage(d.Message, ui.MessageUrgencyWarn)
		return
	}
	if len(d.MassRenameSources) == 0 {
		a.closeFileDialog()
		return
	}
	entries := make([]localfs.Entry, len(d.MassRenameSources))
	for i, s := range d.MassRenameSources {
		entries[i] = localfs.Entry{Name: s.Name, Path: s.Path, Type: localfs.EntryFile}
	}
	panelPath := a.activePanel().Path
	find, replace := "", ""
	if len(d.Fields) > 0 {
		find = d.Fields[0].Value
	}
	if len(d.Fields) > 1 {
		replace = d.Fields[1].Value
	}
	mode := ops.MassRenameModeSimple
	var rx *regexp.Regexp
	if d.MassRenameMode == ui.MassRenameModeUIRegex {
		mode = ops.MassRenameModeRegex
		if strings.TrimSpace(find) != "" {
			var err error
			rx, err = ops.MassRenameCompileRegex(find)
			if err != nil {
				msg := ops.MassRenameRegexCompileUserMessage(err)
				if msg == "" {
					msg = err.Error()
				}
				a.setTransientMessage(msg, ui.MessageUrgencyWarn)
				return
			}
		}
	}
	caseFold := d.MassRenameCaseFold && d.MassRenameMode == ui.MassRenameModeUISimple
	rows, err := ops.MassRenameCompute(entries, panelPath, mode, find, replace, caseFold, rx)
	if err != nil {
		a.setTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	if vErr := ops.MassRenameValidateRows(rows); vErr != nil {
		a.setTransientMessage(vErr.Error(), ui.MessageUrgencyWarn)
		return
	}
	if !ops.MassRenameHasWork(rows) {
		a.setTransientMessage("Nothing to rename", ui.MessageUrgencyInfo)
		return
	}
	if err := ops.ExecuteMassRename(rows); err != nil {
		a.setErrorMessage("Mass rename failed", err)
		a.closeFileDialog()
		return
	}
	n := 0
	for _, r := range rows {
		if r.NewBase != r.OldBase {
			n++
		}
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.activePanel().ClearSelection()
	a.setTransientMessage(fmt.Sprintf("Renamed %d file(s)", n), ui.MessageUrgencyInfo)
}
