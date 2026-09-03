package dialog

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Delete confirmation totals always walk mount points (e.g. Samba) even when panel
// disk-usage metering keeps the listing-volume gate enabled by default.
const deleteDialogDescendIntoMountPoints = true

// ClearDeleteDialogReconcileCache resets the delete dialog's disk-usage scan reconcile cache.
// Called when the file dialog closes (of any type) and before opening a fresh delete dialog.
func (h *Handler) ClearDeleteDialogReconcileCache() {
	h.deleteDialogScanFP = ""
	h.deleteDialogSelGen = 0
	h.deleteDialogPanelPath = ""
	h.deleteDialogPrunedPaths = nil
	h.deleteDialogScanArmedGen = 0
	h.deleteDialogScanArmedPath = ""
	h.deleteDialogScanDebounce.Invalidate()
}

// DeleteDialogScanFP returns the last enqueued directory-set fingerprint for the delete
// confirmation dialog's disk-usage scan (empty when no scan has been enqueued).
func (h *Handler) DeleteDialogScanFP() string {
	return h.deleteDialogScanFP
}

func (h *Handler) snapshotDeleteDialogSource(p *panel.State) ([]string, bool) {
	if h.deleteDialogSelGen == p.SelectionDerivedGen() && h.deleteDialogPanelPath == p.PathString() && h.deleteDialogPrunedPaths != nil {
		return h.deleteDialogPrunedPaths, true
	}
	source, err := ops.ResolveSource(p)
	if err != nil {
		return nil, false
	}
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	h.deleteDialogSelGen = p.SelectionDerivedGen()
	h.deleteDialogPanelPath = p.PathString()
	h.deleteDialogPrunedPaths = pruned
	return pruned, true
}

// DeleteDialogSummary formats the impact summary line ("N files (size)") for source resolved
// against panel p.
func (h *Handler) DeleteDialogSummary(p *panel.State, source ops.Source) string {
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	return h.deleteDialogSummaryFromPruned(p, pruned)
}

func (h *Handler) deleteDialogSummaryFromPruned(p *panel.State, pruned []string) string {
	byPath := p.EntriesByPath()
	remote := p.Path.IsRemote()
	files, bytes, pending := ui.PathsDeleteImpact(pruned, byPath, remote, h.diskUsage)
	return ui.FormatDeleteImpactSummary(files, bytes, pending, h.host.Styles().SymbolWorking())
}

func (h *Handler) invalidateDeleteDialogDiskCache(p *panel.State, source ops.Source) {
	if h.diskUsage == nil || p.Path.IsRemote() {
		return
	}
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	byPath := p.EntriesByPath()
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
		h.diskUsage.InvalidateSubtree(path)
	}
}

// RefreshDeleteDialogSummary recomputes the open delete dialog's impact summary from the
// current disk-usage cache. No-op when the delete dialog is not open.
func (h *Handler) RefreshDeleteDialogSummary() {
	if !h.deleteDialogOpen() {
		return
	}
	p := h.host.ActivePanel()
	pruned, ok := h.snapshotDeleteDialogSource(p)
	if !ok {
		return
	}
	h.model.FileDialog.DeleteSummary = h.deleteDialogSummaryFromPruned(p, pruned)
}

// ReconcileDeleteDialogScans enqueues a disk-usage scan for the open delete dialog's
// directories when the selection or panel has changed since the last enqueue. No-op when the
// delete dialog is not open or disk-usage metering is disabled.
//
// Which directories still need scanning is computed on a debounced background goroutine
// (see ApplyDeleteDialogScanNeed) since diskusage.DirectoriesNeedingScan stats every pruned
// directory not yet cached — for a delete confirmation over a large selection that's too much
// synchronous work for the main goroutine, same as the panel/find-dialog selection-size scans.
func (h *Handler) ReconcileDeleteDialogScans() {
	if h.diskUsage == nil || h.model.ViewMode != ui.ViewBrowser {
		return
	}
	if !h.deleteDialogOpen() {
		h.ClearDeleteDialogReconcileCache()
		return
	}
	p := h.host.ActivePanel()
	if p.Path.IsRemote() {
		h.deleteDialogScanFP = ""
		h.deleteDialogScanDebounce.Invalidate()
		return
	}
	pruned, ok := h.snapshotDeleteDialogSource(p)
	if !ok {
		h.deleteDialogScanFP = ""
		h.deleteDialogScanDebounce.Invalidate()
		return
	}
	gen := p.SelectionDerivedGen()
	path := p.PathString()
	if h.deleteDialogScanArmedGen == gen && h.deleteDialogScanArmedPath == path {
		return
	}
	h.deleteDialogScanArmedGen = gen
	h.deleteDialogScanArmedPath = path

	// PruneNestedPaths always returns a fresh slice (snapshotDeleteDialogSource replaces
	// deleteDialogPrunedPaths wholesale, never mutates it in place), but copy defensively
	// before crossing the goroutine boundary, matching the panel/find-dialog reconcilers.
	prunedCopy := append([]string(nil), pruned...)
	byPath := p.EntriesByPath()
	listingDev := p.ListingDevice
	listingDevValid := p.ListingDeviceValid
	engine := h.diskUsage
	ignore := h.diskUsageIgnore
	screen := h.screen
	delay := time.Duration(h.host.Config().UI.SelectionSizeScanDebounceMS) * time.Millisecond

	h.deleteDialogScanDebounce.Arm(delay, func() {
		need := diskusage.DirectoriesNeedingScan(
			prunedCopy, byPath, listingDev, listingDevValid,
			engine, deleteDialogDescendIntoMountPoints, ignore,
		)
		_ = screen.PostEvent(tcell.NewEventInterrupt(DeleteDialogScanNeedPayload{Need: need, Gen: gen, Path: path}))
	})
}

// ApplyDeleteDialogScanNeed applies the result of a background ReconcileDeleteDialogScans pass:
// starts the actual (already-async) disk-usage walk for whatever still needs scanning, unless
// the delete dialog closed or its source selection changed again since the scan was computed.
func (h *Handler) ApplyDeleteDialogScanNeed(d DeleteDialogScanNeedPayload) {
	if h.diskUsage == nil || !h.deleteDialogOpen() {
		return
	}
	p := h.host.ActivePanel()
	if p.SelectionDerivedGen() != d.Gen || p.PathString() != d.Path {
		return
	}
	fp := strings.Join(d.Need, "\n")
	if fp == "" {
		h.deleteDialogScanFP = ""
		h.RefreshDeleteDialogSummary()
		return
	}
	if fp == h.deleteDialogScanFP {
		return
	}
	h.deleteDialogScanFP = fp
	h.diskUsage.StartScanFromListing(
		d.Need,
		h.diskUsageIgnore,
		h.model.ActivePanel,
		diskusage.ListingVolumeGate{},
	)
}

func (h *Handler) deleteDialogOpen() bool {
	return h.model.FileDialog.Open &&
		h.model.FileDialog.DialogType == dialog.FileDialogDelete &&
		!h.model.FileDialog.DeleteDanglingDirs
}
