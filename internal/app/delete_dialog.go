package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Delete confirmation totals always walk mount points (e.g. Samba) even when panel
// disk-usage metering keeps the listing-volume gate enabled by default.
const deleteDialogDescendIntoMountPoints = true

func (a *App) clearDeleteDialogReconcileCache() {
	a.deleteDialogScanFP = ""
	a.deleteDialogSelGen = 0
	a.deleteDialogPanelPath = ""
	a.deleteDialogPrunedPaths = nil
}

func (a *App) snapshotDeleteDialogSource(p *panel.State) ([]string, bool) {
	if a.deleteDialogSelGen == p.SelectionDerivedGen() && a.deleteDialogPanelPath == p.PathString() && a.deleteDialogPrunedPaths != nil {
		return a.deleteDialogPrunedPaths, true
	}
	source, err := ops.ResolveSource(p)
	if err != nil {
		return nil, false
	}
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	a.deleteDialogSelGen = p.SelectionDerivedGen()
	a.deleteDialogPanelPath = p.PathString()
	a.deleteDialogPrunedPaths = pruned
	return pruned, true
}

func (a *App) deleteDialogSummary(p *panel.State, source ops.Source) string {
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	return a.deleteDialogSummaryFromPruned(p, pruned)
}

func (a *App) deleteDialogSummaryFromPruned(p *panel.State, pruned []string) string {
	byPath := entriesByPath(p)
	remote := p.Path.IsRemote()
	files, bytes, pending := ui.PathsDeleteImpact(
		pruned,
		byPath,
		remote,
		p.ListingDevice,
		p.ListingDeviceValid,
		a.diskUsage,
		deleteDialogDescendIntoMountPoints,
		a.diskUsageIgnore,
	)
	return ui.FormatDeleteImpactSummary(files, bytes, pending, a.styles.SymbolWorking())
}

func (a *App) invalidateDeleteDialogDiskCache(p *panel.State, source ops.Source) {
	if a.diskUsage == nil || p.Path.IsRemote() {
		return
	}
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	byPath := entriesByPath(p)
	for _, path := range pruned {
		entry, found := byPath[path]
		if !found {
			var err error
			entry, err = localfs.EntryFromPath(path)
			if err != nil || entry.Type != localfs.EntryDirectory {
				continue
			}
		} else if entry.Type != localfs.EntryDirectory {
			continue
		}
		a.diskUsage.InvalidateSubtree(path)
	}
}

func entriesByPath(p *panel.State) map[string]localfs.Entry {
	byPath := make(map[string]localfs.Entry, len(p.Entries))
	for _, e := range p.Entries {
		byPath[e.Path] = e
	}
	return byPath
}

func (a *App) refreshDeleteDialogSummary() {
	if !a.deleteDialogOpen() {
		return
	}
	p := a.activePanel()
	pruned, ok := a.snapshotDeleteDialogSource(p)
	if !ok {
		return
	}
	a.model.FileDialog.DeleteSummary = a.deleteDialogSummaryFromPruned(p, pruned)
}

func (a *App) reconcileDeleteDialogScans() {
	if a.diskUsage == nil || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if !a.deleteDialogOpen() {
		a.clearDeleteDialogReconcileCache()
		return
	}
	p := a.activePanel()
	if p.Path.IsRemote() {
		a.deleteDialogScanFP = ""
		return
	}
	pruned, ok := a.snapshotDeleteDialogSource(p)
	if !ok {
		a.deleteDialogScanFP = ""
		return
	}
	byPath := entriesByPath(p)
	need := directoriesNeedingScan(
		pruned,
		byPath,
		p.ListingDevice,
		p.ListingDeviceValid,
		a.diskUsage,
		deleteDialogDescendIntoMountPoints,
		a.diskUsageIgnore,
	)
	fp := strings.Join(need, "\n")
	if fp == "" {
		a.deleteDialogScanFP = ""
		a.refreshDeleteDialogSummary()
		return
	}
	if fp == a.deleteDialogScanFP {
		return
	}
	a.deleteDialogScanFP = fp
	panelID := a.model.ActivePanel
	a.diskUsage.StartScanFromListing(
		need,
		a.diskUsageIgnore,
		panelID,
		diskusage.ListingVolumeGate{},
	)
}
