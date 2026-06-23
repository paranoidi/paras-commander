package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// massRenameFindFieldFocus is FocusedField for the Find / Pattern input (0–2 are mode radios).
const massRenameFindFieldFocus = 3

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
			ap = filepath.Join(p.PathString(), ap)
		}
		ap = filepath.Clean(ap)
		sources = append(sources, ui.MassRenameSource{Path: ap, Name: filepath.Base(ap)})
	}
	fields := []ui.FileDialogField{
		{Label: "Find", Value: "", Cursor: 0},
		{Label: "Replace", Value: "", Cursor: 0},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:                       true,
		DialogType:                 ui.FileDialogMassRename,
		Fields:                     fields,
		FocusedField:               massRenameFindFieldFocus,
		MassRenameMode:             ui.MassRenameModeUISimple,
		MassRenameCaseFold:         false,
		MassRenameShowOnlyModified: false,
		MassRenamePreviewScroll:    0,
		MassRenameSources:          sources,
		MassRenamePreviewBefore:    nil,
		MassRenamePreviewAfter:     nil,
	}
	a.massRenameSyncFieldLabels()
	a.recomputeMassRenamePreview()
}

func (a *App) massRenameSyncFieldLabels() {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogMassRename || len(d.Fields) < 2 {
		return
	}
	if d.MassRenameMode == ui.MassRenameModeUIExternalEditor {
		return // no fields visible in external editor mode
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
	d.MassRenameReplacementSyntaxHint = ""
	if len(d.Fields) > 0 {
		d.Fields[0].InputInvalid = false
	}
	if len(d.MassRenameSources) == 0 {
		d.MassRenamePreviewBefore = nil
		d.MassRenamePreviewAfter = nil
		d.MassRenamePreviewBeforeRemoved = nil
		d.MassRenamePreviewBeforeReplaced = nil
		d.MassRenamePreviewAfterAdded = nil
		return
	}

	if d.MassRenameMode == ui.MassRenameModeUIExternalEditor {
		a.recomputeMassRenameExternalEditorPreview()
		return
	}

	entries := make([]localfs.Entry, len(d.MassRenameSources))
	for i, s := range d.MassRenameSources {
		entries[i] = localfs.Entry{Name: s.Name, Path: s.Path, Type: localfs.EntryFile}
	}
	panelPath := a.activePanel().PathString()
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
				if strings.TrimSpace(d.MassRenamePatternCompileHint) == "" {
					d.MassRenamePatternCompileHint = strings.TrimSpace(err.Error())
				}
				rx = nil // preview as no-op transform; error is shown on the pattern field only
			}
		}
	}
	d.MassRenameReplacementSyntaxHint = ops.MassRenameReplacementSyntaxHint(rx)
	caseFold := d.MassRenameCaseFold && d.MassRenameMode == ui.MassRenameModeUISimple
	rows, err := ops.MassRenameCompute(entries, panelPath, mode, find, replace, caseFold, rx)
	if err != nil {
		d.Message = err.Error()
		d.MassRenamePreviewBefore = []string{"! " + err.Error()}
		d.MassRenamePreviewAfter = []string{""}
		d.MassRenamePreviewBeforeRemoved = nil
		d.MassRenamePreviewBeforeReplaced = nil
		d.MassRenamePreviewAfterAdded = nil
		return
	}
	if len(d.Fields) > 0 && !d.Fields[0].InputInvalid && !ops.MassRenameFindMatchesAny(rows, mode, find, caseFold, rx) {
		d.Fields[0].InputInvalid = true
	}
	rowErrs := ops.MassRenameRowErrors(rows)
	before := make([]string, 0, len(rows))
	after := make([]string, 0, len(rows))
	beforeRemoved := make([][]search.Range, 0, len(rows))
	beforeReplaced := make([][]search.Range, 0, len(rows))
	afterAdded := make([][]search.Range, 0, len(rows))
	afterError := make([]bool, 0, len(rows))
	for i, r := range rows {
		if d.MassRenameShowOnlyModified && r.OldBase == r.NewBase {
			continue
		}
		lcsRemoved, _ := ui.MassRenameDiff(r.OldBase, r.NewBase)
		matchRanges := ops.MassRenameMatchRanges(r.OldBase, mode, find, caseFold, rx)
		removed, replaced := ops.MassRenameBeforePreviewHighlightRanges(lcsRemoved, matchRanges, replace)
		before = append(before, r.OldBase)
		after = append(after, r.NewBase)
		beforeRemoved = append(beforeRemoved, removed)
		beforeReplaced = append(beforeReplaced, replaced)
		afterAdded = append(afterAdded, ops.MassRenameReplacementRanges(r.OldBase, mode, find, replace, caseFold, rx))
		afterError = append(afterError, i < len(rowErrs) && rowErrs[i] != nil)
	}
	d.MassRenamePreviewBefore = before
	d.MassRenamePreviewAfter = after
	d.MassRenamePreviewBeforeRemoved = beforeRemoved
	d.MassRenamePreviewBeforeReplaced = beforeReplaced
	d.MassRenamePreviewAfterAdded = afterAdded
	d.MassRenamePreviewAfterError = afterError
	_, h := a.screen.Size()
	vp := ui.MassRenamePreviewViewportRows(h, d.MassRenameMode)
	ui.MassRenameEnsurePreviewScroll(&a.model.FileDialog, vp, len(before))
}

func (a *App) recomputeMassRenameExternalEditorPreview() {
	d := &a.model.FileDialog
	namesReady := len(d.MassRenameExternalNames) == len(d.MassRenameSources)
	var rowErrs []error
	if namesReady {
		rows := make([]ops.MassRenameRow, len(d.MassRenameSources))
		for i, src := range d.MassRenameSources {
			rows[i] = ops.MassRenameRow{SourcePath: src.Path, OldBase: src.Name, NewBase: d.MassRenameExternalNames[i]}
		}
		rowErrs = ops.MassRenameRowErrors(rows)
	}
	before := make([]string, 0, len(d.MassRenameSources))
	after := make([]string, 0, len(d.MassRenameSources))
	beforeRemoved := make([][]search.Range, 0, len(d.MassRenameSources))
	beforeReplaced := make([][]search.Range, 0, len(d.MassRenameSources))
	afterAdded := make([][]search.Range, 0, len(d.MassRenameSources))
	afterError := make([]bool, 0, len(d.MassRenameSources))
	for i, src := range d.MassRenameSources {
		newBase := src.Name
		if namesReady {
			newBase = d.MassRenameExternalNames[i]
		}
		if d.MassRenameShowOnlyModified && src.Name == newBase {
			continue
		}
		before = append(before, src.Name)
		after = append(after, newBase)
		beforeRemoved = append(beforeRemoved, nil)
		beforeReplaced = append(beforeReplaced, nil)
		if namesReady {
			_, added := ui.MassRenameDiff(src.Name, newBase)
			afterAdded = append(afterAdded, added)
		} else {
			afterAdded = append(afterAdded, nil)
		}
		afterError = append(afterError, i < len(rowErrs) && rowErrs[i] != nil)
	}
	d.MassRenamePreviewBefore = before
	d.MassRenamePreviewAfter = after
	d.MassRenamePreviewBeforeRemoved = beforeRemoved
	d.MassRenamePreviewBeforeReplaced = beforeReplaced
	d.MassRenamePreviewAfterAdded = afterAdded
	d.MassRenamePreviewAfterError = afterError
	_, h := a.screen.Size()
	vp := ui.MassRenamePreviewViewportRows(h, d.MassRenameMode)
	ui.MassRenameEnsurePreviewScroll(d, vp, len(before))
}

func (a *App) applyMassRenameModeFromFocus() {
	d := &a.model.FileDialog
	switch d.FocusedField {
	case 0:
		d.MassRenameMode = ui.MassRenameModeUISimple
	case 1:
		d.MassRenameMode = ui.MassRenameModeUIRegex
	case 2:
		d.MassRenameMode = ui.MassRenameModeUIExternalEditor
	}
	if d.MassRenameMode == ui.MassRenameModeUIRegex && d.FocusedField == 6 {
		d.FocusedField = 5 // Simple's show-modified (6) → Regex's show-modified (5)
	}
	a.massRenameSyncFieldLabels()
	a.recomputeMassRenamePreview()
}

func (a *App) tryRejectMassRenameOK(d *ui.FileDialogState) bool {
	if d.DialogType != ui.FileDialogMassRename || ui.FileDialogMassRenameOKEnabled(*d) {
		return false
	}
	msg := a.massRenameOKBlockedMessage(d)
	if msg == "" {
		msg = "Mass rename cannot be applied"
	}
	a.setTransientMessage(msg, ui.MessageUrgencyCritical)
	return true
}

func (a *App) massRenameOKBlockedMessage(d *ui.FileDialogState) string {
	if d.MassRenameMode == ui.MassRenameModeUIExternalEditor {
		if len(d.MassRenameExternalNames) == 0 {
			return "Launch the editor first (Enter on External $EDITOR option)"
		}
	} else if len(d.Fields) > 0 && d.Fields[0].InputInvalid {
		find := d.Fields[0].Value
		if d.MassRenameMode == ui.MassRenameModeUIRegex {
			if _, err := ops.MassRenameCompileRegex(find); err != nil {
				msg := ops.MassRenameRegexCompileUserMessage(err)
				if msg != "" {
					return msg
				}
				return strings.TrimSpace(err.Error())
			}
			if hint := strings.TrimSpace(d.MassRenamePatternCompileHint); hint != "" {
				return hint
			}
		}
		return "No selected file names match"
	}
	rows, err := a.massRenameComputeRows(d)
	if err != nil {
		return err.Error()
	}
	if vErr := ops.MassRenameValidateRows(rows); vErr != nil {
		return vErr.Error()
	}
	return ""
}

func (a *App) massRenameComputeRows(d *ui.FileDialogState) ([]ops.MassRenameRow, error) {
	if len(d.MassRenameSources) == 0 {
		return nil, fmt.Errorf("no files to rename")
	}
	if d.MassRenameMode == ui.MassRenameModeUIExternalEditor {
		if len(d.MassRenameExternalNames) == 0 {
			return nil, fmt.Errorf("launch the editor first (Enter on External $EDITOR)")
		}
		rows := make([]ops.MassRenameRow, len(d.MassRenameSources))
		for i, src := range d.MassRenameSources {
			newBase := src.Name
			if i < len(d.MassRenameExternalNames) {
				newBase = d.MassRenameExternalNames[i]
			}
			rows[i] = ops.MassRenameRow{SourcePath: src.Path, OldBase: src.Name, NewBase: newBase}
		}
		return rows, nil
	}
	entries := make([]localfs.Entry, len(d.MassRenameSources))
	for i, s := range d.MassRenameSources {
		entries[i] = localfs.Entry{Name: s.Name, Path: s.Path, Type: localfs.EntryFile}
	}
	panelPath := a.activePanel().PathString()
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
				return nil, err
			}
		}
	}
	caseFold := d.MassRenameCaseFold && d.MassRenameMode == ui.MassRenameModeUISimple
	return ops.MassRenameCompute(entries, panelPath, mode, find, replace, caseFold, rx)
}

func (a *App) executeMassRename() {
	d := &a.model.FileDialog
	a.recomputeMassRenamePreview()
	if a.tryRejectMassRenameOK(d) {
		return
	}
	if len(d.MassRenameSources) == 0 {
		a.closeFileDialog()
		return
	}
	rows, err := a.massRenameComputeRows(d)
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

func (a *App) massRenameEditorFooterEligible() bool {
	d := &a.model.FileDialog
	return d.Open && d.DialogType == ui.FileDialogMassRename
}

func (a *App) launchMassRenameExternalEditor() {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogMassRename {
		return
	}
	d.MassRenameMode = ui.MassRenameModeUIExternalEditor
	a.massRenameSyncFieldLabels()

	// Write source names to a temp file, one per line.
	tmp, err := os.CreateTemp("", "paras-rename-*.txt")
	if err != nil {
		a.setTransientMessage(fmt.Sprintf("Cannot create temp file: %v", err), ui.MessageUrgencyCritical)
		return
	}
	tmpPath := tmp.Name()
	for i, src := range d.MassRenameSources {
		line := src.Name
		if i < len(d.MassRenameSources)-1 {
			line += "\n"
		}
		if _, werr := tmp.WriteString(line); werr != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			a.setTransientMessage(fmt.Sprintf("Cannot write temp file: %v", werr), ui.MessageUrgencyCritical)
			return
		}
	}
	// Ensure last name also has a newline so editors don't complain.
	if len(d.MassRenameSources) > 0 {
		_, _ = tmp.WriteString("\n")
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		a.setTransientMessage(fmt.Sprintf("Cannot write temp file: %v", err), ui.MessageUrgencyCritical)
		return
	}
	defer func() { _ = os.Remove(tmpPath) }()

	launchErr := a.withTerminalReleased(func() error {
		return externalEditorRunner(context.Background(), tmpPath)
	})
	if launchErr != nil {
		a.setTransientMessage(fmt.Sprintf("Editor error: %v", launchErr), ui.MessageUrgencyCritical)
		return
	}

	raw, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		a.setTransientMessage(fmt.Sprintf("Cannot read temp file: %v", readErr), ui.MessageUrgencyCritical)
		return
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	// Remove trailing empty element that Split produces when the file ends with a newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) != len(d.MassRenameSources) {
		d.MassRenameExternalNames = nil
		a.setTransientMessage(
			fmt.Sprintf("Line count mismatch: got %d, expected %d", len(lines), len(d.MassRenameSources)),
			ui.MessageUrgencyCritical,
		)
		a.recomputeMassRenamePreview()
		return
	}
	d.MassRenameExternalNames = lines
	a.recomputeMassRenamePreview()
}
