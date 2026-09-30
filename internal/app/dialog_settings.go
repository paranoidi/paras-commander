package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/dialogform"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
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
	cols := a.model.MetaResults[panelID]
	n := min(len(cols), len(panel.SortDialogRadios()))
	metaRadios := make([]dialog.SortDialogMetaRadio, n)
	metaNames := make([]string, n)
	for i := 0; i < n; i++ {
		metaRadios[i] = dialog.SortDialogMetaRadio{Title: cols[i].ColumnTitle, Name: cols[i].EntryName}
		metaNames[i] = cols[i].EntryName
	}
	sortMode := target.Sort.Mode
	metaColumn := target.Sort.MetaColumn
	if sortMode == panel.SortMeta && !slices.Contains(metaNames, metaColumn) {
		// The panel's current meta sort column isn't among the (capped) dialog list — fall
		// back to Name rather than show a radio selection with nothing checked.
		sortMode = panel.SortName
		metaColumn = ""
	}
	a.model.SortDialog = dialog.SortDialogState{
		Open:                  true,
		SortMode:              sortMode,
		SortReverse:           target.Sort.Reverse,
		DirectoriesFirst:      target.Sort.DirectoriesFirst,
		DiskUsageIdleSizeSort: target.Sort.DiskUsageIdleSizeSort,
		MetaRadios:            metaRadios,
		MetaColumn:            metaColumn,
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
		MetaColumn:            a.model.SortDialog.MetaColumn,
	}, a.panelViewportRows(a.model.SortDialog.PanelID))
	// An explicit apply always sorts immediately; any meta idle-resort timer left over from
	// before this apply (same or different column, resolved or not) is now stale.
	a.invalidateMetaIdleSortPanel(a.model.SortDialog.PanelID)
	a.setTransientMessage(fmt.Sprintf("Sort: %s", target.Sort.Mode.String()), ui.MessageUrgencyInfo)
	a.closeSortDialog()
}

func (a *App) handleSortDialogKey(event *tcell.EventKey) {
	st := &a.model.SortDialog
	n := st.MetaCount()
	cb := st.CheckboxFocus() // first checkbox: disk usage idle sort
	// Segments: sort mode + meta radios(0..cb-1) | options checkboxes(cb..cb+2) | buttons(cb+3..cb+4).
	form := dialog.NewDialogLinearForm(cb+3).WithSegments(0, cb, cb+3)
	a.handleLinearFormDialogKey(event, form, dialogform.Handlers{
		Focus:              &st.Focus,
		OnMoveFocus:        st.MoveFocus,
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
				st.Focus = cb
			case 'r', 'R':
				st.SortReverse = !st.SortReverse
				st.Focus = cb + 1
			case 'd', 'D':
				st.DirectoriesFirst = !st.DirectoriesFirst
				st.Focus = cb + 2
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
			if focus >= len(radios) && focus < len(radios)+n {
				// No mnemonics for meta radios: shortcut letters are already taken by the
				// built-in radios, and meta column names are user-defined.
				st.SortMode = panel.SortMeta
				st.MetaColumn = st.MetaRadios[focus-len(radios)].Name
				return true
			}
			switch {
			case focus == cb:
				st.DiskUsageIdleSizeSort = !st.DiskUsageIdleSizeSort
			case focus == cb+1:
				st.SortReverse = !st.SortReverse
			case focus == cb+2:
				st.DirectoriesFirst = !st.DirectoriesFirst
			case focus == form.OKIndex():
				a.applySortDialog()
			case focus == form.CancelIndex():
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
		UseNerdfontIcons:       a.config.UI.UseNerdfontIcons,
		ZoomActivePanel:        a.config.UI.Zoom.ActivePanel,
		PaneSplitStacked:       a.config.UI.Zoom.Orientation == config.PaneSplitStacked,
		ScrollMode:             panel.EffectiveScrollMode(sm),
		PanelScrollbar:         uiscrollbar.EffectiveStyle(sb),
		PanelScrollbarInactive: a.config.UI.Scroll.ScrollbarInactive,
		ListFormat:             panel.EffectiveListFormat(lf),
		Focus:                  0,
	}
	def := config.DefaultCarouselSplit()
	for i := range a.model.ConfigDialog.Split {
		v := a.config.Carousel.Split[i]
		a.model.ConfigDialog.Split[i] = dialog.FileDialogField{Value: v, Prefill: def[i], Cursor: utf8.RuneCountInString(v)}
	}
}

func (a *App) closeConfigDialog() {
	a.model.ConfigDialog.Open = false
}

func (a *App) applyConfigDialog() {
	split := make([]string, len(a.model.ConfigDialog.Split))
	for i, f := range a.model.ConfigDialog.Split {
		split[i] = strings.TrimSpace(f.Value)
	}
	if _, err := panelcarousel.ParseLayout(split, a.config.Carousel.ShowSize); err != nil {
		for i, tok := range split {
			if !panelcarousel.ValidSplitToken(tok, i) {
				a.model.ConfigDialog.Focus = 12 + i
				break
			}
		}
		a.setErrorMessage("Carousel split", err)
		return
	}
	a.zoomActivePanelOverride = nil
	a.paneSplitOrientationOverride = nil
	val := a.model.ConfigDialog.UseNerdfontIcons
	zoom := a.model.ConfigDialog.ZoomActivePanel
	paneSplit := config.PaneSplitSideBySide
	if a.model.ConfigDialog.PaneSplitStacked {
		paneSplit = config.PaneSplitStacked
	}
	scrollMode := panel.ScrollModeTOMLValue(a.model.ConfigDialog.ScrollMode)
	sb := uiscrollbar.TOMLValue(a.model.ConfigDialog.PanelScrollbar)
	lf := panel.EffectiveListFormat(a.model.ConfigDialog.ListFormat)
	a.config.UI.UseNerdfontIcons = val
	a.setStyles(a.styles)
	a.config.UI.Zoom.ActivePanel = zoom
	a.config.UI.Zoom.Orientation = paneSplit
	a.config.UI.Scroll.Mode = scrollMode
	a.config.UI.Scroll.Scrollbar = sb
	a.config.Panels.DefaultListingFormat = panel.ListingFormatTOMLValue(lf)
	a.config.Carousel.Split = split
	a.model.CarouselLayout = carouselLayoutFromConfig(a.config.Carousel)
	a.model.UseNerdfontIcons = val
	a.model.PanelScrollbar = uiscrollbar.EffectiveStyle(a.model.ConfigDialog.PanelScrollbar)
	a.model.Primary.ListFormat = lf
	a.model.Secondary.ListFormat = lf
	a.syncScrollFromConfig()
	a.closeConfigDialog()
	msg := "Configuration saved"
	patch := map[string]any{
		"ui": map[string]any{
			"use_nerdfont_icons": val,
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
		"carousel": map[string]any{"split": split},
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
	if st.ResetDefaultsConfirm {
		a.handleConfigResetDefaultsConfirmKey(event)
		return
	}
	if event.Key() == tcell.KeyF9 {
		a.editConfigTOMLFromDialog()
		return
	}
	if event.Key() == tcell.KeyF8 && a.configFileExists() {
		st.ResetDefaultsConfirm = true
		st.ResetDefaultsConfirmFocus = 0
		return
	}
	if i, ok := dialog.ConfigDialogSplitIndex(st.Focus); ok && !dialog.AltDialogOK(event) && !dialog.AltDialogCancel(event) {
		if dialog.HandleFileDialogFieldKey(event, &st.Split[i], a.keys.DialogInput, nil) {
			return
		}
	}
	// Segments: view checkboxes(0-2) | scroll section(3-8) | listing radios(9-11) | split inputs(12,13,14 each) | buttons(15).
	form := dialog.NewDialogLinearForm(15).WithSegments(0, 3, 9, 12, 13, 14, 15)
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
					st.Focus = 9 + i
					return true
				}
			}
			switch r {
			case 'd', 'D':
				st.UseNerdfontIcons = !st.UseNerdfontIcons
				st.Focus = 0
			case 'z', 'Z':
				st.ZoomActivePanel = !st.ZoomActivePanel
				st.Focus = 1
			case 'h', 'H':
				st.PaneSplitStacked = !st.PaneSplitStacked
				st.Focus = 2
			default:
				return false
			}
			return true
		},
		OnSpace: func(focus int) bool {
			switch focus {
			case 0:
				st.UseNerdfontIcons = !st.UseNerdfontIcons
			case 1:
				st.ZoomActivePanel = !st.ZoomActivePanel
			case 2:
				st.PaneSplitStacked = !st.PaneSplitStacked
			case 9, 10, 11:
				st.ListFormat = listRadios[focus-9].Format
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

// handleConfigResetDefaultsConfirmKey handles Yes/No for the "delete config.toml and reset to
// defaults?" confirmation shown from the Configuration dialog's F8 handler.
func (a *App) handleConfigResetDefaultsConfirmKey(event *tcell.EventKey) {
	st := &a.model.ConfigDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'y', 'Y':
			st.ResetDefaultsConfirm = false
			a.resetConfigToDefaults()
			return
		case 'n', 'N':
			st.ResetDefaultsConfirm = false
			return
		}
	}
	switch event.Key() {
	case tcell.KeyEsc:
		st.ResetDefaultsConfirm = false
	case tcell.KeyLeft:
		st.ResetDefaultsConfirmFocus = dialog.DialogPairLeftRight(st.ResetDefaultsConfirmFocus, false)
	case tcell.KeyRight:
		st.ResetDefaultsConfirmFocus = dialog.DialogPairLeftRight(st.ResetDefaultsConfirmFocus, true)
	case tcell.KeyEnter:
		yes := st.ResetDefaultsConfirmFocus == 0
		st.ResetDefaultsConfirm = false
		if yes {
			a.resetConfigToDefaults()
		}
	}
}

// configFileExists reports whether config.toml currently exists on disk.
func (a *App) configFileExists() bool {
	if a.paths.ConfigFile == "" {
		return false
	}
	_, err := os.Stat(a.paths.ConfigFile)
	return err == nil
}

// resetConfigToDefaults deletes config.toml and reloads the (now default) configuration,
// then rebuilds the Configuration dialog to reflect it.
func (a *App) resetConfigToDefaults() {
	path := a.paths.ConfigFile
	if path == "" {
		a.setErrorMessage("Reset to defaults", fmt.Errorf("config.toml location is unknown"))
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		a.setErrorMessage("Reset to defaults", err)
		return
	}
	cfg, err := config.LoadFromPaths(a.paths)
	if err != nil {
		a.openConfigDialog()
		a.setErrorMessage("Reset to defaults", err)
		return
	}
	a.config = cfg
	a.restartStatusCommandTicker(cfg.StatusCommand)
	a.openConfigDialog()
	a.setTransientMessage("Configuration reset to defaults", ui.MessageUrgencyInfo)
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
	a.restartStatusCommandTicker(cfg.StatusCommand)
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
		FullPath:           false,
		MetaColumnCount:    metaCount,
		IncludeMetaColumns: metaCount > 0,
		Focus:              dialog.GroupSelectFocusPattern,
	}
	a.groupSelectPreviewKey = groupSelectPreviewKey{}
}

func (a *App) closeGroupSelect() {
	a.findCtrl.StopGroupCount()
	a.model.GroupSelect.Open = false
	a.model.GroupSelect.Text = ""
	a.model.GroupSelect.PatternCompileHint = ""
	a.model.GroupSelect.PreviewShow = false
	a.groupSelectPreviewKey = groupSelectPreviewKey{}
}

// groupSelectPreviewKey is the comparable subset of dialog.GroupSelectState that determines
// the find-context live preview count, used by updateGroupSelectPreview to skip re-arming the
// async count debounce when nothing that affects matching has changed (e.g. cursor movement).
type groupSelectPreviewKey struct {
	Text          string
	Mode          string
	PatternMode   panel.GroupPatternMode
	FilesOnly     bool
	DirsOnly      bool
	CaseSensitive bool
	FullPath      bool
}

// groupSelectMeta builds the meta-column match data for the panel-context group-select dialog
// from its current include/only-meta checkboxes. Shared by executeGroupSelect and
// updateGroupSelectPreview so the two stay in sync.
func (a *App) groupSelectMeta(gs *dialog.GroupSelectState) panel.GroupSelectMeta {
	return ui.MetaMatchData(a.model.MetaResults[a.model.ActivePanel], gs.IncludeMetaColumns, gs.OnlyMetaColumns)
}

// updateGroupSelectPreview recomputes the group-select dialog's live result preview (matched
// file/folder counts that would actually change selection state) from the current pattern and
// options. No-op when the dialog is closed; hides the preview when the pattern is empty or
// fails to compile.
func (a *App) updateGroupSelectPreview() {
	gs := &a.model.GroupSelect
	if !gs.Open || gs.Text == "" {
		gs.PreviewShow = false
		a.groupSelectPreviewKey = groupSelectPreviewKey{}
		return
	}
	if _, err := panel.NewGroupMatcher(gs.Text, gs.PatternMode, gs.CaseSensitive); err != nil {
		gs.PreviewShow = false
		a.groupSelectPreviewKey = groupSelectPreviewKey{}
		return
	}
	selectMode := gs.Mode == "select"
	context := gs.Context
	if context == "" {
		context = "panel"
	}
	if context == "find" {
		key := groupSelectPreviewKey{
			Text:          gs.Text,
			Mode:          gs.Mode,
			PatternMode:   gs.PatternMode,
			FilesOnly:     gs.FilesOnly,
			DirsOnly:      gs.DirsOnly,
			CaseSensitive: gs.CaseSensitive,
			FullPath:      gs.FullPath,
		}
		if key == a.groupSelectPreviewKey {
			if files, dirs, changed := a.findCtrl.ExtendGroupCount(); changed {
				gs.PreviewFiles = files
				gs.PreviewFolders = dirs
				gs.PreviewShow = true
			}
			return
		}
		a.groupSelectPreviewKey = key
		gs.PreviewShow = false
		a.findCtrl.StartGroupCount(findctrl.GroupSelectRequest{
			Mode:          findctrl.GroupSelectMode(gs.Mode),
			Pattern:       gs.Text,
			FilesOnly:     gs.FilesOnly,
			DirsOnly:      gs.DirsOnly,
			CaseSensitive: gs.CaseSensitive,
			FullPath:      gs.FullPath,
			PatternMode:   gs.PatternMode,
		}, !selectMode)
		return
	}
	p := a.activePanel()
	files, dirs, err := p.CountGroupMatches(gs.Text, gs.FilesOnly, gs.DirsOnly, gs.CaseSensitive, gs.PatternMode, a.groupSelectMeta(gs), !selectMode)
	if err != nil {
		gs.PreviewShow = false
		return
	}
	gs.PreviewFiles = files
	gs.PreviewFolders = dirs
	gs.PreviewShow = true
}

// applyGroupCountPayload applies an async find-context group-select count (StartGroupCount)
// result. Returns false (no redraw needed) when the dialog has since closed, switched away
// from find context, or the payload is stale (superseded by a newer StartGroupCount call).
func (a *App) applyGroupCountPayload(p findctrl.GroupCountPayload) bool {
	gs := &a.model.GroupSelect
	if !gs.Open || gs.Context != "find" {
		return false
	}
	files, dirs, ok := a.findCtrl.HandleGroupCount(p)
	if !ok {
		return false
	}
	gs.PreviewFiles = files
	gs.PreviewFolders = dirs
	gs.PreviewShow = true
	return true
}

func (a *App) groupSelectForm() dialog.DialogLinearForm {
	n := 8
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
			FullPath:      gs.FullPath,
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
			case 'y', 'Y':
				a.toggleGroupSelectField(gs, dialog.GroupSelectFocusFilesOnly)
				gs.Focus = dialog.GroupSelectFocusFilesOnly
			case 't', 'T':
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
			case 'a', 'A':
				if a.toggleGroupSelectField(gs, dialog.GroupSelectFocusFullPath) {
					gs.Focus = dialog.GroupSelectFocusFullPath
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
	if focus, ok := dialog.GroupSelectMoveFocus(gs.Focus, event.Key(), gs.PatternMode, gs.MetaColumnCount, dialog.GroupSelectShowsFullPath(*gs)); ok {
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
	case dialog.GroupSelectFocusFullPath:
		if !dialog.GroupSelectShowsFullPath(*gs) {
			return false
		}
		gs.FullPath = !gs.FullPath
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
	case 'y', 'Y', 't', 'T', 'e', 'E', 'r', 'R', 's', 'S', 'i', 'I', 'm', 'M', 'n', 'N', 'a', 'A':
		return true
	default:
		return false
	}
}
