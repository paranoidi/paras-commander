package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// previewHost implements apphandler/preview.Host for *App.
type previewHost struct {
	appShellHost
}

func (h previewHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h previewHost) InactivePanelID() int { return h.app.inactivePanelID() }

func (h previewHost) ActiveViewportRows() int { return h.app.activeViewportRows() }

func (h previewHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }

func (h previewHost) SelectionsStripViewportRows(panelID int) int {
	return h.app.selectionsStripViewportRows(panelID)
}

func (h previewHost) InQuickFilterUI() bool { return h.app.inQuickFilterUI() }

func (h previewHost) LaunchedAsFileViewer() bool { return h.app.launchedFileViewer }

func (h previewHost) SwitchPanel() { h.app.switchPanel() }

func (h previewHost) SyncFollowTargetPath(driver *panel.State) (string, bool) {
	return h.app.syncFollowTargetPath(driver)
}

func (h previewHost) PanelSyncFollowHeldListNav(resolvedAction string, event *tcell.EventKey) bool {
	return h.app.panelSyncFollowHeldListNav(resolvedAction, event)
}

func (h previewHost) ArmPanelSyncFollowNavCoalesceAfterListNav() {
	h.app.armPanelSyncFollowNavCoalesceAfterListNav()
}

func (h previewHost) ClearPanelSyncFollowNavCoalesce() { h.app.clearPanelSyncFollowNavCoalesce() }

func (h previewHost) ArmCursorNameHintNavCoalesceAfterListNav() {
	h.app.armCursorNameHintNavCoalesceAfterListNav()
}

func (h previewHost) PathVolumeContendsWithActiveJob(path string) bool {
	return h.app.pathVolumeContendsWithActiveJob(path)
}

func (h previewHost) GitStatusScheduler(panelID int) panel.GitStatusScheduler {
	return h.app.gitStatusScheduler(panelID)
}

func (h previewHost) AsyncLoadScheduler(panelID int) panel.AsyncLoadScheduler {
	return h.app.asyncLoadScheduler(panelID)
}

func (h previewHost) ScheduleCarouselParentSnapshot(panelID int, viewportRows int) {
	h.app.scheduleCarouselParentSnapshot(panelID, viewportRows)
}

func (h previewHost) ScheduleCarouselChildSnapshot(panelID int, viewportRows int) {
	h.app.scheduleCarouselChildSnapshot(panelID, viewportRows)
}

func (h previewHost) PeekGitStatus(workRoot, listDir string, paths []gitstatus.ListingPaths) (map[string]gitstatus.Cell, bool) {
	if h.app.gitStatusCache == nil {
		return nil, false
	}
	return h.app.gitStatusCache.PeekStatusesForListing(workRoot, listDir, paths)
}

func (h previewHost) EffectivePaneSplitOrientation() ui.SplitOrientation {
	return h.app.effectivePaneSplitOrientation()
}

func (h previewHost) PanelPaneSplit(width int, filePreviewOpen bool) ui.PanelPaneSplit {
	return h.app.panelPaneSplit(width, filePreviewOpen)
}

func (h previewHost) LayoutForTerminalSizePreview(width, height int, filePreviewOpen bool) ui.Layout {
	return h.app.layoutForTerminalSizePreview(width, height, filePreviewOpen)
}

func (h previewHost) TerminalLayoutRows() int { return h.app.terminalLayoutRows() }

func (h previewHost) BrowserMenuDefinitions() []menu.Definition {
	return h.app.browserMenuDefinitions()
}

func (h previewHost) CarouselAutohideInactivePanel() bool {
	return h.app.carouselAutohideInactivePanel()
}

func (h previewHost) EditActiveFile() { h.app.editActiveFile() }

func (h previewHost) EditFullscreenPreviewFile() { h.app.editFullscreenPreviewFile() }

func (h previewHost) OpenDeleteDialogForPreviewedFile() {
	h.app.dialogCtrl.OpenDeleteDialogForPreviewedFile()
}

func (h previewHost) OpenPreviewLeaderMenu() { h.app.togglePreviewLeaderMenu() }

func (h previewHost) HandleFileDialogFieldKey(ev *tcell.EventKey, f *dialog.FileDialogField, afterEdit func()) bool {
	return dialog.HandleFileDialogFieldKey(ev, f, h.app.keys.DialogInput, afterEdit)
}

func (h previewHost) PersistPartial(patch map[string]interface{}) error {
	return h.app.persistPartial(patch)
}

func (h previewHost) Config() config.Config { return h.app.config }

func (h previewHost) Styles() theme.Theme { return h.app.styles }

func (h previewHost) SetPreviewStyle(style string) { h.app.config.Preview.Style = style }

// ApplyPreviewStyle validates, persists, and reports the final Chroma style selection (F3 style
// picker Enter). Mutates the live App config directly, not a handler-side snapshot.
func (h previewHost) ApplyPreviewStyle(name string) bool {
	validated := config.NormalizePreviewStyle(name)
	h.app.config.Preview.Style = validated
	msg := fmt.Sprintf("Preview style changed to %s", validated)
	urgency := ui.MessageUrgencyInfo
	if err := h.app.persistPartial(map[string]interface{}{
		"preview": map[string]interface{}{
			"style": validated,
		},
	}); err != nil {
		msg = fmt.Sprintf("%s (config save failed: %v)", msg, err)
		urgency = ui.MessageUrgencyWarn
	}
	h.app.setTransientMessage(msg, urgency)
	return true
}

func (h previewHost) SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range) {
	return syncFilteredListRanks(lines, query, matchRangeSlots, caseInsensitive)
}

func (h previewHost) ClampFilteredListSelection(selected *int, rankedLen int) {
	clampFilteredListSelection(selected, rankedLen)
}

func (h previewHost) HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool {
	return handleFilteredListSelectionKey(ev, focus, selected, rankedLen, listRows, ensureScroll)
}

var _ previewctrl.Host = previewHost{}
