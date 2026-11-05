package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// openIconLegendDialog replaces the help dialog with the file-list icon legend.
func (a *App) openIconLegendDialog() {
	a.closeHelpDialog()
	a.model.IconLegendDialog = dialog.IconLegendDialogState{Open: true, Items: a.fileListIconLegend()}
}

func (a *App) closeIconLegendDialog() {
	a.model.IconLegendDialog = dialog.IconLegendDialogState{}
}

func (a *App) handleIconLegendKey(event *tcell.EventKey) {
	st := &a.model.IconLegendDialog
	w, h := a.screen.Size()
	spec, ok := dialog.IconLegendDialogView(*st, a.layoutForTerminalSize(w, h), a.styles)
	if !ok {
		return
	}
	if ev, ok := dlg.HandleKey(spec, &st.Focus, event); ok {
		a.applyIconLegendEvent(ev)
	}
}

// applyIconLegendEvent acts on a widget event of the legend (key or click).
func (a *App) applyIconLegendEvent(ev dlg.Event) {
	switch {
	case ev.Kind == dlg.EventListMove:
		a.model.IconLegendDialog.Scroll = ev.Offset
	case ev.ID == dialog.IconLegendClose:
		a.closeIconLegendDialog()
	}
}

// fileListIconLegend lists the file-list row icons with the theme's glyph and the style the row
// painters use (panellist suffix styles, FolderIcon colours). Fileindex states have their own
// legend (fileIndexLegend).
func (a *App) fileListIconLegend() []primitive.LegendItem {
	th := a.styles
	rows := []struct {
		icon  string
		style tcell.Style
		label string
	}{
		{th.FolderIcon(theme.FolderIconOpen), th.PanelIconFolderOpen, "open in other panel"},
		{th.FolderIcon(theme.FolderIconMount), th.PanelIconFolderMount, "mount point"},
		{th.FolderIcon(theme.FolderIconScanning), th.StatusScanning, "sizing folder"},
		{th.FolderIcon(theme.FolderIconExcluded), th.PanelIconFolderExcluded, "excluded from size"},
		{string(th.IconFilelistPreviewLoading()), th.StatusScanning, "loading preview"},
		{string(th.IconFilelistJob()), th.PanelRowMarkJob, "job writing"},
		{string(th.IconFilelistJob()), th.PanelRowMarkJobRead, "job reading"},
		{string(th.IconFilelistJob()), th.StatusDecision, "job awaits decision"},
		{string(th.IconFilelistQueued()), th.StatusQueued, "job queued"},
		{string(th.IconFilelistMove()), th.PanelRowMarkJobMove, "will move"},
		{string(th.IconFilelistDelete()), th.PanelRowMarkJobDelete, "will delete"},
		{string(th.IconFilelistMoveRemoving()), th.PanelRowMarkJobRemoving, "removing moved source"},
		{string(th.IconFilelistNew()), th.PanelRowMarkNew, "new"},
		{string(th.IconFilelistNew()), th.PanelRowMarkNewPrevious, "new, previous batch"},
		{string(th.IconFilelistNew()), th.PanelRowMarkNewOlder, "new, older"},
		{string(th.IconFilelistRenamed()), th.PanelRowMarkRenamed, "renamed"},
		{string(th.IconFilelistWorking()), th.StatusScanning, "loading"},
		{string(th.IconPinRune()), th.PanelRowMarkPinned, "pinned"},
		{string(th.IconFilelistRangeAnchor()), th.PanelRowMarkRangeAnchor, "range start"},
		{string(th.IconFilelistOtherMatch()), th.PanelRowMarkOtherMatch, "also in other panel"},
		{string(th.IconFilelistOtherMatch()), th.PanelRowMarkOtherMatchCursor, "cursor file in other panel"},
		{string(th.IconFilelistSelectionSubtree()), th.PanelRowMarkSelectionSubtree, "selection inside"},
		{string(th.IconFilelistNoPermission()), th.PanelRowMarkNoPermission, "no permission"},
	}
	legend := make([]primitive.LegendItem, 0, len(rows))
	for _, r := range rows {
		if r.icon != "" && r.icon != "\x00" {
			legend = append(legend, primitive.LegendItem{Icon: r.icon, Style: r.style, Label: r.label})
		}
	}
	return legend
}
