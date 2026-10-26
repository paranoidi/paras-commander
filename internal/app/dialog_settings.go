package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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
	if ev, ok := dlg.HandleKey(dialog.SortDialogView(*st), &st.Focus, event); ok {
		a.applySortDialogEvent(ev)
	}
}

// applySortDialogEvent acts on a widget event of the sort dialog (key or click).
func (a *App) applySortDialogEvent(ev dlg.Event) {
	st := &a.model.SortDialog
	switch ev.ID {
	case dlg.OK:
		a.applySortDialog()
	case dlg.Cancel:
		a.closeSortDialog()
	default:
		st.Apply(ev)
	}
}

func (a *App) handleListingFormatDialogKey(event *tcell.EventKey) {
	st := &a.model.ListingFormatDialog
	if ev, ok := dlg.HandleKey(dialog.ListingFormatView(*st), &st.Focus, event); ok {
		a.applyListingFormatEvent(ev)
	}
}

// applyListingFormatEvent acts on a widget event of the listing format dialog (key or click).
func (a *App) applyListingFormatEvent(ev dlg.Event) {
	st := &a.model.ListingFormatDialog
	switch ev.ID {
	case dlg.OK:
		a.applyListingFormatDialog()
	case dlg.Cancel:
		a.closeListingFormatDialog()
	default:
		st.Apply(ev)
	}
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
		Mouse:                  a.config.UI.Mouse,
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
				a.model.ConfigDialog.Focus = dialog.ConfigSplitID(i)
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
	a.config.UI.Mouse = a.model.ConfigDialog.Mouse
	a.applyMouseConfig()
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
			"mouse":              a.config.UI.Mouse,
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
		st.ResetDefaultsConfirmFocus = dialog.ConfigConfirmYes
		return
	}
	termW, _ := a.screen.Size()
	spec := dialog.ConfigDialogView(*st, termW)
	if ev, handled := dlg.HandleKey(spec, &st.Focus, event); handled {
		a.applyConfigDialogEvent(ev)
		return
	}
	// The engine declined: text keys go to the focused carousel split input.
	if i, ok := dialog.ConfigSplitIndex(dlg.Focused(spec, st.Focus)); ok {
		dialog.HandleFileDialogFieldKey(event, &st.Split[i], a.keys.DialogInput, nil)
	}
}

// applyConfigDialogEvent acts on a widget event of the Configuration dialog (key or click),
// including its Yes/No sub-confirms.
func (a *App) applyConfigDialogEvent(ev dlg.Event) {
	st := &a.model.ConfigDialog
	switch {
	case st.EditStubConfirm:
		a.applyConfigEditStubConfirmEvent(ev)
	case st.ResetDefaultsConfirm:
		a.applyConfigResetDefaultsConfirmEvent(ev)
	case ev.Kind == dlg.EventCaret:
		if i, ok := dialog.ConfigSplitIndex(ev.ID); ok {
			st.Split[i].SetCursor(ev.Index)
		}
	case ev.ID == dlg.OK:
		a.applyConfigDialog()
	case ev.ID == dlg.Cancel:
		a.closeConfigDialog()
	case ev.ID != "":
		if _, isInput := dialog.ConfigSplitIndex(ev.ID); isInput {
			a.applyConfigDialog() // Enter in a split input confirms
		} else {
			st.Apply(ev)
		}
	}
}

// handleConfigEditStubConfirmKey handles Yes/No for the "config.toml does not exist, generate
// default and open it?" confirmation shown from the Configuration dialog's F9 handler.
func (a *App) handleConfigEditStubConfirmKey(event *tcell.EventKey) {
	st := &a.model.ConfigDialog
	if ev, ok := dlg.HandleKey(dialog.ConfigEditStubView(), &st.EditStubConfirmFocus, event); ok {
		a.applyConfigEditStubConfirmEvent(ev)
	}
}

func (a *App) applyConfigEditStubConfirmEvent(ev dlg.Event) {
	st := &a.model.ConfigDialog
	if ev.ID == "" {
		return
	}
	st.EditStubConfirm = false
	if ev.ID == dialog.ConfigConfirmYes {
		a.createAndEditConfigStub()
	}
}

// handleConfigResetDefaultsConfirmKey handles Yes/No for the "delete config.toml and reset to
// defaults?" confirmation shown from the Configuration dialog's F8 handler.
func (a *App) handleConfigResetDefaultsConfirmKey(event *tcell.EventKey) {
	st := &a.model.ConfigDialog
	if ev, ok := dlg.HandleKey(dialog.ConfigResetDefaultsView(), &st.ResetDefaultsConfirmFocus, event); ok {
		a.applyConfigResetDefaultsConfirmEvent(ev)
	}
}

func (a *App) applyConfigResetDefaultsConfirmEvent(ev dlg.Event) {
	st := &a.model.ConfigDialog
	if ev.ID == "" {
		return
	}
	st.ResetDefaultsConfirm = false
	if ev.ID == dialog.ConfigConfirmYes {
		a.resetConfigToDefaults()
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
	a.applyMouseConfig()
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
			st.EditStubConfirmFocus = dialog.ConfigConfirmYes
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
	a.applyMouseConfig()
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
		Focus:              dialog.PatternInput,
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
	Types         [dialog.FilterTypeCount]bool
}

// groupSelectMeta builds the meta-column match data for the panel-context group-select dialog
// from its current include/only-meta checkboxes. Shared by executeGroupSelect and
// updateGroupSelectPreview so the two stay in sync.
func (a *App) groupSelectMeta(gs *dialog.GroupSelectState) panel.GroupSelectMeta {
	return ui.MetaMatchData(a.model.MetaResults[a.model.ActivePanel], gs.IncludeMetaColumns, gs.OnlyMetaColumns)
}

// groupSelectSpec is the panel.FilterSpec for the dialog's current state (Invert unused).
func (a *App) groupSelectSpec(gs *dialog.GroupSelectState) panel.FilterSpec {
	spec := panel.FilterSpec{
		Pattern: gs.Text, Mode: gs.PatternMode, CaseSensitive: gs.CaseSensitive,
		FilesOnly: gs.FilesOnly, DirsOnly: gs.DirsOnly,
	}
	spec.Types, spec.TypeLabel = dialog.FilterTypesRegexp(gs.DirsOnly, gs.Types, a.config.Filter.Types)
	return spec
}

// groupSelectRequest is the find/output-context request for the dialog's current state.
func (a *App) groupSelectRequest(gs *dialog.GroupSelectState) findctrl.GroupSelectRequest {
	spec := a.groupSelectSpec(gs)
	return findctrl.GroupSelectRequest{
		Mode:          findctrl.GroupSelectMode(gs.Mode),
		Pattern:       gs.Text,
		FilesOnly:     gs.FilesOnly,
		DirsOnly:      gs.DirsOnly,
		CaseSensitive: gs.CaseSensitive,
		FullPath:      gs.FullPath,
		PatternMode:   gs.PatternMode,
		Types:         spec.Types,
		TypeLabel:     spec.TypeLabel,
	}
}

// groupSelectEmpty reports whether the dialog restricts nothing (no pattern, no types), in
// which case there is nothing to preview or apply.
func (a *App) groupSelectEmpty() bool {
	return a.groupSelectSpec(&a.model.GroupSelect).Empty()
}

// updateGroupSelectPreview recomputes the group-select dialog's live result preview (matched
// file/folder counts that would actually change selection state) from the current pattern and
// options. No-op when the dialog is closed; hides the preview when the pattern is empty or
// fails to compile.
func (a *App) updateGroupSelectPreview() {
	gs := &a.model.GroupSelect
	if !gs.Open || a.groupSelectEmpty() {
		gs.PreviewShow = false
		a.groupSelectPreviewKey = groupSelectPreviewKey{}
		return
	}
	if gs.Text != "" {
		if _, err := panel.NewGroupMatcher(gs.Text, gs.PatternMode, gs.CaseSensitive); err != nil {
			gs.PreviewShow = false
			a.groupSelectPreviewKey = groupSelectPreviewKey{}
			return
		}
	}
	selectMode := gs.Mode == "select"
	context := gs.Context
	if context == "" {
		context = "panel"
	}
	if context == "output" {
		matches, err := a.outputGroupMatches(gs)
		gs.PreviewFiles, gs.PreviewFolders, gs.PreviewShow = 0, 0, err == nil
		for _, m := range matches {
			if m.IsDir {
				gs.PreviewFolders++
			} else {
				gs.PreviewFiles++
			}
		}
		return
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
			Types:         gs.Types,
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
		a.findCtrl.StartGroupCount(a.groupSelectRequest(gs), !selectMode)
		return
	}
	p := a.activePanel()
	files, dirs, err := p.CountGroupMatches(a.groupSelectSpec(gs), a.groupSelectMeta(gs), !selectMode)
	if err != nil {
		gs.PreviewShow = false
		return
	}
	gs.PreviewFiles = files
	gs.PreviewFolders = dirs
	gs.PreviewShow = true
}

// outputGroupMatches returns the output dialog's detected paths (deduped) that gs matches and
// whose selection state in the active panel would flip (select: not yet selected; unselect:
// selected). In-memory only: IsDir was captured at detection.
func (a *App) outputGroupMatches(gs *dialog.GroupSelectState) ([]dialog.OutputPathMatch, error) {
	req := a.groupSelectRequest(gs)
	match, err := findctrl.NewPathMatcher(req)
	if err != nil {
		return nil, err
	}
	p := a.activePanel()
	selectMode := req.Mode == findctrl.GroupSelectModeSelect
	var out []dialog.OutputPathMatch
	for _, m := range a.model.CommandOutputDialog.UniquePaths() {
		if (gs.DirsOnly && !m.IsDir) || (gs.FilesOnly && m.IsDir) ||
			p.IsSelectedPath(m.Path) == selectMode || !match(m.Path) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// applyOutputGroupSelect selects or unselects the matching detected output paths in the active
// panel and reports the result.
func (a *App) applyOutputGroupSelect(gs *dialog.GroupSelectState) {
	matches, err := a.outputGroupMatches(gs)
	if err != nil {
		a.setTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	if len(matches) == 0 {
		a.setTransientMessage("No matches", ui.MessageUrgencyWarn)
		return
	}
	req := a.groupSelectRequest(gs)
	p := a.activePanel()
	verb, conflicts := "Unselected", false
	if req.Mode == findctrl.GroupSelectModeSelect {
		verb = "Selected"
		paths := make([]string, len(matches))
		isDir := make(map[string]bool, len(matches))
		for i, m := range matches {
			paths[i], isDir[m.Path] = m.Path, m.IsDir
		}
		conflicts = p.BulkAddSelections(paths, func(path string) bool { return isDir[path] })
	} else {
		// Every match is selected, so toggling only removes and never conflicts.
		for _, m := range matches {
			p.TogglePathSelection(m.Path)
		}
	}
	a.setTransientMessage(verb+" matching "+req.Describe(), ui.MessageUrgencyInfo)
	if conflicts {
		a.setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
	}
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

// groupSelectView is the group select dialog's widget spec for the current terminal size.
func (a *App) groupSelectView() dlg.Spec {
	termW, termH := a.screen.Size()
	return dialog.GroupSelectView(a.model.GroupSelect, dialog.Layout{Width: termW, Height: termH}, a.styles)
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
	if a.groupSelectEmpty() {
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
		a.findCtrl.ApplyGroupSelect(a.groupSelectRequest(gs))
	case "output":
		a.applyOutputGroupSelect(gs)
	default:
		p := a.activePanel()
		meta := a.groupSelectMeta(gs)
		spec := a.groupSelectSpec(gs)
		what := findctrl.GroupSelectRequest{Pattern: gs.Text, TypeLabel: spec.TypeLabel}.Describe()
		var err error
		var matched bool
		if gs.Mode == "select" {
			matched, err = p.SelectGroup(spec, meta)
			if err == nil {
				if matched {
					a.setTransientMessage("Selected matching "+what, ui.MessageUrgencyInfo)
				} else {
					a.setTransientMessage("No matches", ui.MessageUrgencyWarn)
				}
			}
		} else {
			matched, err = p.UnselectGroup(spec, meta)
			if err == nil {
				if matched {
					a.setTransientMessage("Unselected matching "+what, ui.MessageUrgencyInfo)
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
	if gs.Focus == dialog.PatternInput {
		e := scrollquery.NewEdit(&gs.Text, &gs.TextCursor, &gs.TextScroll, a.groupSelectQueryWidth(), nil)
		e.Apply()
	}
	if a.tryRejectGroupSelectOK() {
		return
	}
	a.executeGroupSelect()
}

// handleGroupSelectKey routes a key for the open group select dialog. The engine owns focus and
// the mnemonics; the pattern input's text editing runs when the engine declines a key.
func (a *App) handleGroupSelectKey(event *tcell.EventKey) {
	dialogctrl.HandlePatternDialogKey(a.groupSelectPatternKeys(), event)
}

// groupSelectPatternKeys binds the open group select dialog to the shared pattern dialog handling.
func (a *App) groupSelectPatternKeys() dialogctrl.PatternKeys {
	gs := &a.model.GroupSelect
	return dialogctrl.PatternKeys{
		Spec: a.groupSelectView(), Focus: &gs.Focus,
		Text: &gs.Text, Cursor: &gs.TextCursor, Scroll: &gs.TextScroll, QueryWidth: a.groupSelectQueryWidth,
		Keys:    a.keys.DialogInput,
		Confirm: a.confirmGroupSelectFromInput, Cancel: a.closeGroupSelect,
		Apply: func(ev dlg.Event) { gs.Apply(ev) },
	}
}
