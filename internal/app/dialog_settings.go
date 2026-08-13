package app

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode"

	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/dialogform"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Sort dialog handlers

func (a *App) openSortDialog() {
	a.openSortDialogForPanel(a.model.ActivePanel)
}

func (a *App) openSortDialogForPanel(panelID int) {
	a.closeListingFormatDialog()
	target := a.panelByID(panelID)
	a.model.SortDialog = dialog.SortDialogState{
		Open:                  true,
		SortMode:              target.Sort.Mode,
		SortReverse:           target.Sort.Reverse,
		DirectoriesFirst:      target.Sort.DirectoriesFirst,
		DiskUsageIdleSizeSort: target.Sort.DiskUsageIdleSizeSort,
		Focus:                 0,
		PanelID:               panelID,
	}
}

func (a *App) closeSortDialog() {
	a.model.SortDialog.Open = false
}

func (a *App) applySortDialog() {
	target := a.panelByID(a.model.SortDialog.PanelID)
	target.ApplySortFromDialog(panel.SortState{
		Mode:                  a.model.SortDialog.SortMode,
		Reverse:               a.model.SortDialog.SortReverse,
		DirectoriesFirst:      a.model.SortDialog.DirectoriesFirst,
		DiskUsageIdleSizeSort: a.model.SortDialog.DiskUsageIdleSizeSort,
	}, a.panelViewportRows(a.model.SortDialog.PanelID))
	a.setTransientMessage(fmt.Sprintf("Sort: %s", target.Sort.Mode.String()), ui.MessageUrgencyInfo)
	a.closeSortDialog()
}

func (a *App) handleSortDialogKey(event *tcell.EventKey) {
	// Segments: sort mode radios(0-3) | options checkboxes(4-6) | buttons(7).
	form := dialog.NewDialogLinearForm(7).WithSegments(0, 4, 7)
	st := &a.model.SortDialog
	a.handleLinearFormDialogKey(event, form, dialogform.Handlers{
		Focus:              &st.Focus,
		OnApply:            a.applySortDialog,
		OnCancel:           a.closeSortDialog,
		AllowPlainOKCancel: true,
		OnMnemonic: func(r rune) bool {
			for i, row := range panel.SortDialogRadios() {
				if unicode.ToLower(r) == unicode.ToLower(row.Shortcut) {
					st.SortMode = row.Mode
					st.Focus = i
					return true
				}
			}
			switch r {
			case 'u', 'U':
				st.DiskUsageIdleSizeSort = !st.DiskUsageIdleSizeSort
				st.Focus = 4
			case 'r', 'R':
				st.SortReverse = !st.SortReverse
				st.Focus = 5
			case 'd', 'D':
				st.DirectoriesFirst = !st.DirectoriesFirst
				st.Focus = 6
			default:
				return false
			}
			return true
		},
		OnSpace: func(focus int) bool {
			radios := panel.SortDialogRadios()
			if focus >= 0 && focus < len(radios) {
				st.SortMode = radios[focus].Mode
				return true
			}
			switch focus {
			case 4:
				st.DiskUsageIdleSizeSort = !st.DiskUsageIdleSizeSort
			case 5:
				st.SortReverse = !st.SortReverse
			case 6:
				st.DirectoriesFirst = !st.DirectoriesFirst
			case form.OKIndex():
				a.applySortDialog()
			case form.CancelIndex():
				a.closeSortDialog()
			default:
				return false
			}
			return true
		},
	})
}

func listingFormatFromShortcut(ch rune, focus *int) (panel.ListFormat, bool) {
	for i, row := range panel.ListFormatDialogRadios() {
		if unicode.ToLower(ch) == unicode.ToLower(row.Shortcut) {
			*focus = i
			return row.Format, true
		}
	}
	return 0, false
}

func scrollModeFromShortcut(ch rune, focus *int) (panel.ScrollMode, bool) {
	for i, row := range panel.ScrollModeDialogRadios() {
		if unicode.ToLower(ch) == unicode.ToLower(row.Shortcut) {
			*focus = dialog.ConfigDialogScrollModeFocus(i)
			return row.Mode, true
		}
	}
	return "", false
}

func panelScrollbarFromShortcut(ch rune, focus *int) (uiscrollbar.Style, bool) {
	for i, row := range uiscrollbar.DialogRadios() {
		if unicode.ToLower(ch) == unicode.ToLower(row.Shortcut) {
			*focus = dialog.ConfigDialogScrollbarFocus(i)
			return row.Style, true
		}
	}
	return "", false
}

func (a *App) handleListingFormatDialogKey(event *tcell.EventKey) {
	// Segments: format radios(0-2) | buttons(3).
	form := dialog.NewDialogLinearForm(3).WithSegments(0, 3)
	st := &a.model.ListingFormatDialog
	radios := panel.ListFormatDialogRadios()
	a.handleLinearFormDialogKey(event, form, dialogform.Handlers{
		Focus:              &st.Focus,
		OnApply:            a.applyListingFormatDialog,
		OnCancel:           a.closeListingFormatDialog,
		AllowPlainOKCancel: true,
		OnMnemonic: func(r rune) bool {
			if format, ok := listingFormatFromShortcut(r, &st.Focus); ok {
				st.ListFormat = format
				return true
			}
			return false
		},
		OnSpace: func(focus int) bool {
			switch focus {
			case 0, 1, 2:
				st.ListFormat = radios[focus].Format
			case form.OKIndex():
				a.applyListingFormatDialog()
			case form.CancelIndex():
				a.closeListingFormatDialog()
			default:
				return false
			}
			return true
		},
	})
}

func (a *App) openListingFormatDialog() {
	a.openListingFormatDialogForPanel(a.model.ActivePanel)
}

func (a *App) openListingFormatDialogForPanel(panelID int) {
	a.closeSortDialog()
	a.clearTransientMessage()
	target := a.panelByID(panelID)
	a.model.ListingFormatDialog = dialog.ListingFormatDialogState{
		Open:       true,
		ListFormat: panel.EffectiveListFormat(target.ListFormat),
		Focus:      0,
		PanelID:    panelID,
	}
}

func (a *App) closeListingFormatDialog() {
	a.model.ListingFormatDialog.Open = false
}

func (a *App) applyListingFormatDialog() {
	st := a.model.ListingFormatDialog
	target := a.panelByID(st.PanelID)
	target.ListFormat = panel.EffectiveListFormat(st.ListFormat)
	a.setTransientMessage(fmt.Sprintf("%s listing: %s", panelLabel(st.PanelID), target.ListFormat.String()), ui.MessageUrgencyInfo)
	a.closeListingFormatDialog()
}

func (a *App) openConfigDialog() {
	a.clearTransientMessage()
	lf, _ := panel.ParseListFormat(a.config.Panels.DefaultListingFormat)
	sm, _ := panel.ParseScrollMode(a.config.UI.Scroll.Mode)
	sb, _ := uiscrollbar.ParseStyle(a.config.UI.Scroll.Scrollbar)
	a.model.ConfigDialog = dialog.ConfigDialogState{
		Open:                   true,
		ShowFileIcons:          a.config.UI.ShowFileIcons,
		ZoomActivePanel:        a.config.UI.Zoom.ActivePanel,
		ShrunkenShowsNameOnly:  a.config.UI.ShrunkenShowsNameOnly,
		PaneSplitStacked:       a.config.UI.Zoom.Orientation == config.PaneSplitStacked,
		ScrollMode:             panel.EffectiveScrollMode(sm),
		PanelScrollbar:         uiscrollbar.EffectiveStyle(sb),
		PanelScrollbarInactive: a.config.UI.Scroll.ScrollbarInactive,
		ListFormat:             panel.EffectiveListFormat(lf),
		Focus:                  0,
	}
}

func (a *App) closeConfigDialog() {
	a.model.ConfigDialog.Open = false
}

func (a *App) applyConfigDialog() {
	a.zoomActivePanelOverride = nil
	a.paneSplitOrientationOverride = nil
	val := a.model.ConfigDialog.ShowFileIcons
	zoom := a.model.ConfigDialog.ZoomActivePanel
	shrunken := a.model.ConfigDialog.ShrunkenShowsNameOnly
	paneSplit := config.PaneSplitSideBySide
	if a.model.ConfigDialog.PaneSplitStacked {
		paneSplit = config.PaneSplitStacked
	}
	scrollMode := panel.ScrollModeTOMLValue(a.model.ConfigDialog.ScrollMode)
	sb := uiscrollbar.TOMLValue(a.model.ConfigDialog.PanelScrollbar)
	lf := panel.EffectiveListFormat(a.model.ConfigDialog.ListFormat)
	a.config.UI.ShowFileIcons = val
	a.config.UI.Zoom.ActivePanel = zoom
	a.config.UI.ShrunkenShowsNameOnly = shrunken
	a.config.UI.Zoom.Orientation = paneSplit
	a.config.UI.Scroll.Mode = scrollMode
	a.config.UI.Scroll.Scrollbar = sb
	a.config.Panels.DefaultListingFormat = panel.ListingFormatTOMLValue(lf)
	a.model.ShowFileIcons = val
	a.model.ShrunkenShowsNameOnly = shrunken
	a.model.PanelScrollbar = uiscrollbar.EffectiveStyle(a.model.ConfigDialog.PanelScrollbar)
	a.model.Primary.ListFormat = lf
	a.model.Secondary.ListFormat = lf
	a.syncScrollFromConfig()
	a.closeConfigDialog()
	msg := "Configuration saved"
	patch := map[string]any{
		"ui": map[string]any{
			"show_file_icons":          val,
			"shrunken_shows_name_only": shrunken,
			"zoom": map[string]any{
				"active_panel": zoom,
				"orientation":  paneSplit,
			},
			"scroll": map[string]any{
				"mode":      scrollMode,
				"scrollbar": sb,
			},
		},
		"panels": map[string]any{
			"default_listing_format": panel.ListingFormatTOMLValue(lf),
		},
	}
	if err := a.persistPartial(patch); err != nil {
		msg = fmt.Sprintf("Configuration saved (could not write config: %v)", err)
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
	a.ensurePanelsVisible()
}

func (a *App) handleConfigDialogKey(event *tcell.EventKey) {
	st := &a.model.ConfigDialog
	if st.EditStubConfirm {
		a.handleConfigEditStubConfirmKey(event)
		return
	}
	if event.Key() == tcell.KeyF9 {
		a.editConfigTOMLFromDialog()
		return
	}
	// Segments: view checkboxes(0-3) | scroll section(4-9) | listing radios(10-12) | buttons(13).
	form := dialog.NewDialogLinearForm(13).WithSegments(0, 4, 10, 13)
	listRadios := panel.ListFormatDialogRadios()
	scrollRadios := panel.ScrollModeDialogRadios()
	sbRadios := uiscrollbar.DialogRadios()
	a.handleLinearFormDialogKey(event, form, dialogform.Handlers{
		Focus:              &st.Focus,
		OnApply:            a.applyConfigDialog,
		OnCancel:           a.closeConfigDialog,
		AllowPlainOKCancel: true,
		OnMoveFocus:        dialog.ConfigDialogMoveScrollFocus,
		OnMnemonic: func(r rune) bool {
			if mode, ok := scrollModeFromShortcut(r, &st.Focus); ok {
				st.ScrollMode = mode
				return true
			}
			if style, ok := panelScrollbarFromShortcut(r, &st.Focus); ok {
				st.PanelScrollbar = style
				return true
			}
			for i, row := range listRadios {
				if unicode.ToLower(r) == unicode.ToLower(row.Shortcut) {
					st.ListFormat = row.Format
					st.Focus = 10 + i
					return true
				}
			}
			switch r {
			case 'f', 'F':
				st.ShowFileIcons = !st.ShowFileIcons
				st.Focus = 0
			case 'z', 'Z':
				st.ZoomActivePanel = !st.ZoomActivePanel
				st.Focus = 1
			case 's', 'S':
				st.ShrunkenShowsNameOnly = !st.ShrunkenShowsNameOnly
				st.Focus = 2
			case 'h', 'H':
				st.PaneSplitStacked = !st.PaneSplitStacked
				st.Focus = 3
			default:
				return false
			}
			return true
		},
		OnSpace: func(focus int) bool {
			switch focus {
			case 0:
				st.ShowFileIcons = !st.ShowFileIcons
			case 1:
				st.ZoomActivePanel = !st.ZoomActivePanel
			case 2:
				st.ShrunkenShowsNameOnly = !st.ShrunkenShowsNameOnly
			case 3:
				st.PaneSplitStacked = !st.PaneSplitStacked
			case 10, 11, 12:
				st.ListFormat = listRadios[focus-10].Format
			case form.OKIndex():
				a.applyConfigDialog()
			case form.CancelIndex():
				a.closeConfigDialog()
			default:
				if idx, ok := dialog.ConfigDialogScrollModeIndex(focus); ok {
					st.ScrollMode = scrollRadios[idx].Mode
					return true
				}
				if idx, ok := dialog.ConfigDialogScrollbarIndex(focus); ok {
					st.PanelScrollbar = sbRadios[idx].Style
					return true
				}
				return false
			}
			return true
		},
	})
}

// handleConfigEditStubConfirmKey handles Yes/No for the "config.toml does not exist, generate
// default and open it?" confirmation shown from the Configuration dialog's F9 handler.
func (a *App) handleConfigEditStubConfirmKey(event *tcell.EventKey) {
	st := &a.model.ConfigDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'y', 'Y':
			st.EditStubConfirm = false
			a.createAndEditConfigStub()
			return
		case 'n', 'N':
			st.EditStubConfirm = false
			return
		}
	}
	switch event.Key() {
	case tcell.KeyEsc:
		st.EditStubConfirm = false
	case tcell.KeyLeft:
		st.EditStubConfirmFocus = dialog.DialogPairLeftRight(st.EditStubConfirmFocus, false)
	case tcell.KeyRight:
		st.EditStubConfirmFocus = dialog.DialogPairLeftRight(st.EditStubConfirmFocus, true)
	case tcell.KeyEnter:
		yes := st.EditStubConfirmFocus == 0
		st.EditStubConfirm = false
		if yes {
			a.createAndEditConfigStub()
		}
	}
}

// editConfigTOMLFromDialog handles F9 in the Configuration dialog: opens config.toml in the
// external editor, or prompts to generate a default stub first when it doesn't exist yet.
func (a *App) editConfigTOMLFromDialog() {
	st := &a.model.ConfigDialog
	if !st.Open {
		return
	}
	path := a.paths.ConfigFile
	if path == "" {
		a.setErrorMessage("Edit config", fmt.Errorf("config.toml location is unknown"))
		return
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			st.EditStubConfirm = true
			st.EditStubConfirmFocus = 0
			return
		}
		a.setErrorMessage("Edit config", err)
		return
	}
	a.openConfigTOMLInEditor(path)
}

// createAndEditConfigStub writes the default config.toml stub (creating the config directory
// if needed, e.g. on a first run) and opens it in the external editor.
func (a *App) createAndEditConfigStub() {
	path := a.paths.ConfigFile
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.setErrorMessage("Edit config", err)
		return
	}
	if err := config.WriteDefaultStub(path); err != nil {
		a.setErrorMessage("Edit config", err)
		return
	}
	a.openConfigTOMLInEditor(path)
}

// openConfigTOMLInEditor launches the external editor on config.toml, then reloads the config
// from disk and rebuilds the Configuration dialog so any manual edits take effect immediately.
func (a *App) openConfigTOMLInEditor(path string) {
	if err := a.openFileInExternalEditor(path); err != nil {
		a.setErrorMessage("Edit config", err)
		return
	}
	cfg, err := config.LoadFromPaths(a.paths)
	if err != nil {
		a.openConfigDialog()
		a.setErrorMessage("Edit config", err)
		return
	}
	a.config = cfg
	a.openConfigDialog()
	a.setTransientMessage("config.toml reloaded", ui.MessageUrgencyInfo)
}

// Group selection dialog handlers

func (a *App) openGroupSelect(mode string, context string) {
	if context == "" {
		context = "panel"
	}
	metaCount := 0
	if context == "panel" {
		metaCount = len(a.model.MetaResults[a.model.ActivePanel])
	}
	a.model.GroupSelect = dialog.GroupSelectState{
		Open:               true,
		Text:               "",
		Mode:               mode,
		Context:            context,
		PatternMode:        panel.GroupPatternShell,
		FilesOnly:          false,
		DirsOnly:           false,
		CaseSensitive:      false,
		MetaColumnCount:    metaCount,
		IncludeMetaColumns: metaCount > 0,
		Focus:              dialog.GroupSelectFocusPattern,
	}
}

func (a *App) closeGroupSelect() {
	a.model.GroupSelect.Open = false
	a.model.GroupSelect.Text = ""
	a.model.GroupSelect.PatternCompileHint = ""
	a.model.GroupSelect.PreviewShow = false
}

// groupSelectMeta builds the meta-column match data for the panel-context group-select dialog
// from its current include/only-meta checkboxes. Shared by executeGroupSelect and
// updateGroupSelectPreview so the two stay in sync.
func (a *App) groupSelectMeta(gs *dialog.GroupSelectState) panel.GroupSelectMeta {
	var meta panel.GroupSelectMeta
	if gs.IncludeMetaColumns && gs.MetaColumnCount > 0 {
		for _, col := range a.model.MetaResults[a.model.ActivePanel] {
			if col.Results != nil {
				meta.Cols = append(meta.Cols, col.Results)
			}
		}
		meta.OnlyMeta = gs.OnlyMetaColumns
	}
	return meta
}

// updateGroupSelectPreview recomputes the group-select dialog's live result preview (matched
// file/folder counts that would actually change selection state) from the current pattern and
// options. No-op when the dialog is closed; hides the preview when the pattern is empty or
// fails to compile.
func (a *App) updateGroupSelectPreview() {
	gs := &a.model.GroupSelect
	if !gs.Open || gs.Text == "" {
		gs.PreviewShow = false
		return
	}
	if _, err := panel.NewGroupMatcher(gs.Text, gs.PatternMode, gs.CaseSensitive); err != nil {
		gs.PreviewShow = false
		return
	}
	selectMode := gs.Mode == "select"
	context := gs.Context
	if context == "" {
		context = "panel"
	}
	var files, dirs int
	if context == "find" {
		files, dirs = a.findCtrl.CountGroupMatches(findctrl.GroupSelectRequest{
			Mode:          findctrl.GroupSelectMode(gs.Mode),
			Pattern:       gs.Text,
			FilesOnly:     gs.FilesOnly,
			DirsOnly:      gs.DirsOnly,
			CaseSensitive: gs.CaseSensitive,
			PatternMode:   gs.PatternMode,
		}, !selectMode)
	} else {
		p := a.activePanel()
		f, d, err := p.CountGroupMatches(gs.Text, gs.FilesOnly, gs.DirsOnly, gs.CaseSensitive, gs.PatternMode, a.groupSelectMeta(gs), !selectMode)
		if err != nil {
			gs.PreviewShow = false
			return
		}
		files, dirs = f, d
	}
	gs.PreviewFiles = files
	gs.PreviewFolders = dirs
	gs.PreviewShow = true
}

func (a *App) groupSelectForm() dialog.DialogLinearForm {
	n := 7
	if a.model.GroupSelect.MetaColumnCount > 0 {
		n += 2 // IncludeMeta + OnlyMeta
	}
	return dialog.NewDialogLinearForm(n)
}

func (a *App) applyGroupSelectModeFromFocus() {
	gs := &a.model.GroupSelect
	switch gs.Focus {
	case dialog.GroupSelectFocusShellRadio:
		gs.PatternMode = panel.GroupPatternShell
	case dialog.GroupSelectFocusRegexRadio:
		gs.PatternMode = panel.GroupPatternRegex
	case dialog.GroupSelectFocusSimpleRadio:
		gs.PatternMode = panel.GroupPatternSimple
	}
	a.groupSelectClampCaseFocus()
}

func (a *App) groupSelectClampCaseFocus() {
	gs := &a.model.GroupSelect
	if gs.PatternMode == panel.GroupPatternRegex && gs.Focus == dialog.GroupSelectFocusCase {
		gs.Focus = dialog.GroupSelectFocusDirsOnly
	}
}

func (a *App) tryRejectGroupSelectOK() bool {
	gs := &a.model.GroupSelect
	if gs.Text == "" {
		return false
	}
	_, err := panel.NewGroupMatcher(gs.Text, gs.PatternMode, gs.CaseSensitive)
	if err == nil {
		return false
	}
	msg := err.Error()
	a.setTransientMessage(msg, ui.MessageUrgencyCritical)
	return true
}

func (a *App) executeGroupSelect() {
	gs := &a.model.GroupSelect
	if gs.Text == "" {
		return
	}
	if a.tryRejectGroupSelectOK() {
		return
	}
	context := gs.Context
	if context == "" {
		context = "panel"
	}
	switch context {
	case "find":
		a.findCtrl.ApplyGroupSelect(findctrl.GroupSelectRequest{
			Mode:          findctrl.GroupSelectMode(gs.Mode),
			Pattern:       gs.Text,
			FilesOnly:     gs.FilesOnly,
			DirsOnly:      gs.DirsOnly,
			CaseSensitive: gs.CaseSensitive,
			PatternMode:   gs.PatternMode,
		})
	default:
		p := a.activePanel()
		meta := a.groupSelectMeta(gs)
		var err error
		var matched bool
		if gs.Mode == "select" {
			matched, err = p.SelectGroup(gs.Text, gs.FilesOnly, gs.DirsOnly, gs.CaseSensitive, gs.PatternMode, meta)
			if err == nil {
				if matched {
					a.setTransientMessage(fmt.Sprintf("Selected matching %q", gs.Text), ui.MessageUrgencyInfo)
				} else {
					a.setTransientMessage("No matches", ui.MessageUrgencyWarn)
				}
			}
		} else {
			matched, err = p.UnselectGroup(gs.Text, gs.FilesOnly, gs.DirsOnly, gs.CaseSensitive, gs.PatternMode, meta)
			if err == nil {
				if matched {
					a.setTransientMessage(fmt.Sprintf("Unselected matching %q", gs.Text), ui.MessageUrgencyInfo)
				} else {
					a.setTransientMessage("No matches", ui.MessageUrgencyWarn)
				}
			}
		}
		if err != nil {
			a.setTransientMessage(err.Error(), ui.MessageUrgencyCritical)
			return
		}
	}
	a.closeGroupSelect()
	if context == "find" && a.model.FindDialog.Open {
		a.paintFindDialogOverlay()
	}
}

// confirmGroupSelectFromInput applies the pattern row then runs OK (Enter / Alt+O).
func (a *App) confirmGroupSelectFromInput() {
	gs := &a.model.GroupSelect
	if gs.Focus == dialog.GroupSelectFocusPattern {
		e := scrollquery.NewEdit(&gs.Text, &gs.TextCursor, &gs.TextScroll, a.groupSelectQueryWidth(), nil)
		e.Apply()
	}
	if gs.Focus >= dialog.GroupSelectFocusShellRadio && gs.Focus <= dialog.GroupSelectFocusSimpleRadio {
		a.applyGroupSelectModeFromFocus()
	}
	if a.tryRejectGroupSelectOK() {
		return
	}
	a.executeGroupSelect()
}

func (a *App) handleGroupSelectKey(event *tcell.EventKey) {
	gs := &a.model.GroupSelect
	form := a.groupSelectForm()

	if dialog.AltDialogOK(event) {
		a.confirmGroupSelectFromInput()
		return
	}
	if dialog.AltDialogCancel(event) {
		a.closeGroupSelect()
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeGroupSelect()
		return
	case tcell.KeyEnter:
		if gs.Focus == form.CancelIndex() {
			a.closeGroupSelect()
			return
		}
		if gs.Focus >= dialog.GroupSelectFocusShellRadio && gs.Focus <= dialog.GroupSelectFocusSimpleRadio {
			a.applyGroupSelectModeFromFocus()
			return
		}
		a.confirmGroupSelectFromInput()
		return
	}

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 's', 'S':
			a.toggleGroupSelectField(gs, dialog.GroupSelectFocusShellRadio)
			return
		case 'r', 'R':
			a.toggleGroupSelectField(gs, dialog.GroupSelectFocusRegexRadio)
			return
		case 'i', 'I':
			a.toggleGroupSelectField(gs, dialog.GroupSelectFocusSimpleRadio)
			return
		}
	}

	if gs.Focus == dialog.GroupSelectFocusPattern {
		skipScrolling := event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) && groupSelectAltIsDialogMnemonic(event.Rune())
		edit := scrollquery.NewEdit(&gs.Text, &gs.TextCursor, &gs.TextScroll, a.groupSelectQueryWidth(), nil)
		if !skipScrolling && a.handleScrollingQueryKey(event, true, edit) {
			return
		}
	}

	switch event.Key() {
	case tcell.KeyRune:
		if keymap.AltLetterModifiers(event.Modifiers()) {
			switch event.Rune() {
			case 'f', 'F':
				a.toggleGroupSelectField(gs, dialog.GroupSelectFocusFilesOnly)
				gs.Focus = dialog.GroupSelectFocusFilesOnly
			case 'd', 'D':
				a.toggleGroupSelectField(gs, dialog.GroupSelectFocusDirsOnly)
				gs.Focus = dialog.GroupSelectFocusDirsOnly
			case 'e', 'E':
				if a.toggleGroupSelectField(gs, dialog.GroupSelectFocusCase) {
					gs.Focus = dialog.GroupSelectFocusCase
				}
			case 'm', 'M':
				if a.toggleGroupSelectField(gs, dialog.GroupSelectFocusIncludeMeta) {
					gs.Focus = dialog.GroupSelectFocusIncludeMeta
				}
			case 'n', 'N':
				if a.toggleGroupSelectField(gs, dialog.GroupSelectFocusOnlyMeta) {
					gs.Focus = dialog.GroupSelectFocusOnlyMeta
				}
			}
			break
		}
		mod := event.Modifiers()
		if mod != tcell.ModNone && mod != tcell.ModShift {
			break
		}
		if dialog.DialogButtonRune(event.Rune()) == dialog.ButtonRuneToggle {
			switch gs.Focus {
			case form.OKIndex():
				a.confirmGroupSelectFromInput()
			case form.CancelIndex():
				a.closeGroupSelect()
			default:
				a.toggleGroupSelectField(gs, gs.Focus)
			}
			break
		}
	}
	if focus, ok := dialog.GroupSelectMoveFocus(gs.Focus, event.Key(), gs.PatternMode, gs.MetaColumnCount); ok {
		gs.Focus = focus
	}
}

// toggleGroupSelectField applies the toggle/radio-set action addressed by focus
// (one of the dialog.GroupSelectFocus* constants). It is the single source of
// truth shared by the Alt-letter mnemonic switch (which sets gs.Focus only when
// the action actually applies) and the Space-toggle switch (which already has
// gs.Focus at the target field). Returns false when focus doesn't address a
// toggleable field, or the field is conditionally hidden (case-sensitivity when
// not shown, meta columns when there are none) so nothing changed.
func (a *App) toggleGroupSelectField(gs *dialog.GroupSelectState, focus int) bool {
	switch focus {
	case dialog.GroupSelectFocusShellRadio:
		gs.PatternMode = panel.GroupPatternShell
		a.groupSelectClampCaseFocus()
	case dialog.GroupSelectFocusRegexRadio:
		gs.PatternMode = panel.GroupPatternRegex
		a.groupSelectClampCaseFocus()
	case dialog.GroupSelectFocusSimpleRadio:
		gs.PatternMode = panel.GroupPatternSimple
	case dialog.GroupSelectFocusFilesOnly:
		gs.FilesOnly = !gs.FilesOnly
		if gs.FilesOnly {
			gs.DirsOnly = false
		}
	case dialog.GroupSelectFocusDirsOnly:
		gs.DirsOnly = !gs.DirsOnly
		if gs.DirsOnly {
			gs.FilesOnly = false
		}
	case dialog.GroupSelectFocusCase:
		if !dialog.GroupSelectShowsCaseSensitive(*gs) {
			return false
		}
		gs.CaseSensitive = !gs.CaseSensitive
	case dialog.GroupSelectFocusIncludeMeta:
		if gs.MetaColumnCount <= 0 {
			return false
		}
		gs.IncludeMetaColumns = !gs.IncludeMetaColumns
		if !gs.IncludeMetaColumns {
			gs.OnlyMetaColumns = false
		}
	case dialog.GroupSelectFocusOnlyMeta:
		if gs.MetaColumnCount <= 0 {
			return false
		}
		gs.OnlyMetaColumns = !gs.OnlyMetaColumns
		if gs.OnlyMetaColumns {
			gs.IncludeMetaColumns = true
		}
	default:
		return false
	}
	return true
}

func groupSelectAltIsDialogMnemonic(r rune) bool {
	switch r {
	case 'f', 'F', 'd', 'D', 'e', 'E', 'r', 'R', 's', 'S', 'i', 'I', 'm', 'M', 'n', 'N':
		return true
	default:
		return false
	}
}
